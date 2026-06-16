package larkbot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/imhistory"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/larksuite/oapi-sdk-go/v3/channel/safety"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

type BotConfig struct {
	AppID         string
	AppSecret     string
	AgentID       int64
	InvokeTimeout int
	// OpenAPIDomain is the Open Platform API base (e.g. https://open.feishu.cn or https://open.larksuite.com).
	OpenAPIDomain string
	// NoAutoStartWS when true, Start() (server boot / global start) skips opening WebSocket for this app;
	// StartBot may still be used from the API for manual per-agent start.
	NoAutoStartWS bool
	// AllowedUsers: Lark open_id values permitted to message this bot. Empty = allow all.
	AllowedUsers []string
}

type BotEntry struct {
	Config    *BotConfig
	Runtime   *agent.Runtime
	wsClient  *ws.Client
	wsCancel  context.CancelFunc
	wsSession uint64
}

var botWSSessionSeq uint64

type Client struct {
	mu       sync.RWMutex
	bots     map[string]*BotEntry
	running  bool
	msgDedup *safety.DedupCache
	outbound *imoutbound.Store
}

var globalClient *Client
var globalRegisterSessions *RegisterAppSessionManager

func Global() *Client {
	if globalClient == nil {
		globalClient = NewClient()
	}
	return globalClient
}

func GlobalRegisterSessions() *RegisterAppSessionManager {
	if globalRegisterSessions == nil {
		globalRegisterSessions = NewRegisterAppSessionManager()
	}
	return globalRegisterSessions
}

func NewClient() *Client {
	return &Client{
		bots:     make(map[string]*BotEntry),
		msgDedup: safety.NewDedupCache(10000, time.Hour),
	}
}

func (c *Client) RegisterBot(cfg *BotConfig, runtime *agent.Runtime) error {
	if cfg == nil || cfg.AppID == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: app_id and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.bots[cfg.AppID]; exists {
		if existing.Config.AgentID == cfg.AgentID {
			existing.Config = cfg
			existing.Runtime = runtime
			logger.Info("larkbot: bot re-registered (same agent, updated config)", "app_id", cfg.AppID, "agent_id", cfg.AgentID)
			return nil
		}
		logger.Warn("larkbot: app_id already bound to another agent, one-to-many not supported",
			"app_id", cfg.AppID, "new_agent_id", cfg.AgentID, "existing_agent_id", existing.Config.AgentID)
		return fmt.Errorf("app_id %s already bound to agent %d, one-to-one only (one-to-many not supported)", cfg.AppID, existing.Config.AgentID)
	}

	c.bots[cfg.AppID] = &BotEntry{
		Config:  cfg,
		Runtime: runtime,
	}

	logger.Info("larkbot: bot registered", "app_id", cfg.AppID, "agent_id", cfg.AgentID)
	return nil
}

func (c *Client) UnregisterBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[appID]
	if ok {
		closeBotWSEntry(entry)
		delete(c.bots, appID)
		logger.Info("larkbot: bot unregistered (ws client closed)", "app_id", appID)
	}
}

func (c *Client) UpdateBotConfig(cfg *BotConfig) error {
	if cfg == nil || cfg.AppID == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: app_id and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.bots[cfg.AppID]
	if !exists {
		return fmt.Errorf("bot not found for app_id: %s", cfg.AppID)
	}

	if entry.Config.AgentID != cfg.AgentID {
		return fmt.Errorf("agent_id mismatch: cannot change agent binding")
	}

	entry.Config = cfg
	logger.Info("larkbot: bot config updated", "app_id", cfg.AppID, "agent_id", cfg.AgentID)
	return nil
}

func (c *Client) newLarkWSClient(cfg *BotConfig) *ws.Client {
	domain := openAPIDomainFromBot(cfg)
	handler := larkdispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			return c.handleMessage(ctx, event)
		}).
		OnP2MessageReactionCreatedV1(func(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
			return c.handleReactionCreated(ctx, event)
		}).
		OnP2MessageReactionDeletedV1(func(ctx context.Context, event *larkim.P2MessageReactionDeletedV1) error {
			return c.handleReactionDeleted(ctx, event)
		})
	return ws.NewClient(cfg.AppID, cfg.AppSecret,
		ws.WithDomain(domain),
		ws.WithLogLevel(larkcore.LogLevelInfo),
		ws.WithAutoReconnect(true),
		ws.WithEventHandler(handler),
	)
}

func (c *Client) RefreshBot(ctx context.Context, appID string) error {
	c.mu.Lock()
	entry, exists := c.bots[appID]
	if !exists || entry == nil {
		c.mu.Unlock()
		return fmt.Errorf("bot not found for app_id: %s", appID)
	}
	closeBotWSEntry(entry)
	cfg := entry.Config
	wsClient := c.newLarkWSClient(cfg)
	ctx2, cancel := context.WithCancel(ctx)
	sess := atomic.AddUint64(&botWSSessionSeq, 1)
	entry.wsClient = wsClient
	entry.wsCancel = cancel
	entry.wsSession = sess
	c.syncRunningFlag()
	c.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("larkbot: ws refresh panic", "app_id", appID, "recover", r)
			}
		}()
		logger.Info("larkbot: refreshing ws client", "app_id", appID)
		err := wsClient.Start(ctx2)
		c.mu.Lock()
		if e, ok := c.bots[appID]; ok && e.wsSession == sess {
			e.wsClient = nil
			e.wsCancel = nil
		}
		c.syncRunningFlag()
		c.mu.Unlock()
		if err != nil {
			logger.Error("larkbot: ws client refresh failed", "app_id", appID, "err", err)
		}
	}()

	return nil
}

func (c *Client) RefreshAllBots(ctx context.Context) {
	appIDs := c.appIDsForGlobalWS("refresh")
	for _, appID := range appIDs {
		if err := c.RefreshBot(ctx, appID); err != nil {
			logger.Warn("larkbot: refresh bot skipped", "app_id", appID, "err", err)
		}
	}
	logger.Info("larkbot: all eligible ws clients refreshed", "bot_count", len(appIDs))
}

// appIDsForGlobalWS returns app_ids that participate in global Start/Refresh (ws_enabled true only).
// Skips NoAutoStartWS bots (ws_enabled false); logs each skip so operators can see why.
func (c *Client) appIDsForGlobalWS(op string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.bots) == 0 {
		return nil
	}
	appIDs := make([]string, 0, len(c.bots))
	for id, e := range c.bots {
		if e != nil && e.Config != nil && e.Config.NoAutoStartWS {
			logger.Info("larkbot: skip global "+op+" (im ws_enabled false)", "app_id", id)
			continue
		}
		appIDs = append(appIDs, id)
	}
	return appIDs
}

func (c *Client) Start(ctx context.Context) {
	appIDs := c.appIDsForGlobalWS("start")
	if len(appIDs) == 0 {
		if c.GetBotCount() == 0 {
			logger.Info("larkbot: no bots registered, skip starting")
		} else {
			logger.Info("larkbot: no bots eligible for global start (all ws_enabled false)")
		}
		return
	}

	for _, appID := range appIDs {
		if err := c.StartBot(ctx, appID); err != nil {
			logger.Warn("larkbot: start bot skipped", "app_id", appID, "err", err)
		}
	}
	logger.Info("larkbot: all eligible ws clients started", "bot_count", len(appIDs))
}

// StartBot opens the WebSocket for a single registered app_id.
func (c *Client) StartBot(parentCtx context.Context, appID string) error {
	c.mu.Lock()
	entry, ok := c.bots[appID]
	if !ok || entry == nil {
		c.mu.Unlock()
		return fmt.Errorf("bot not found for app_id: %s", appID)
	}
	if entry.wsCancel != nil {
		c.mu.Unlock()
		return nil
	}
	cfg := entry.Config
	wsClient := c.newLarkWSClient(cfg)
	ctx, cancel := context.WithCancel(parentCtx)
	sess := atomic.AddUint64(&botWSSessionSeq, 1)
	entry.wsClient = wsClient
	entry.wsCancel = cancel
	entry.wsSession = sess
	c.syncRunningFlag()
	c.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("larkbot: ws start panic", "app_id", appID, "recover", r)
			}
		}()
		logger.Info("larkbot: ws client starting", "app_id", appID)
		err := wsClient.Start(ctx)
		c.mu.Lock()
		if e, ok2 := c.bots[appID]; ok2 && e.wsSession == sess {
			e.wsClient = nil
			e.wsCancel = nil
		}
		c.syncRunningFlag()
		c.mu.Unlock()
		if err != nil {
			logger.Error("larkbot: ws client start failed", "app_id", appID, "err", err)
		}
	}()
	return nil
}

// closeBotWSEntry closes the WS client before cancelling context so auto-reconnect
// cannot race with an in-flight disconnect handler.
func closeBotWSEntry(entry *BotEntry) {
	if entry == nil {
		return
	}
	if entry.wsClient != nil {
		entry.wsClient.Close()
		entry.wsClient = nil
	}
	if entry.wsCancel != nil {
		entry.wsCancel()
		entry.wsCancel = nil
	}
}

// StopBot stops the WebSocket for one app_id without removing registration.
func (c *Client) StopBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.bots[appID]
	if !ok || entry == nil {
		return
	}
	closeBotWSEntry(entry)
	c.syncRunningFlag()
	logger.Info("larkbot: ws client stopped for app", "app_id", appID)
}

func (c *Client) Stop() {
	c.mu.Lock()
	for appID := range c.bots {
		closeBotWSEntry(c.bots[appID])
		logger.Info("larkbot: ws client stopped for app", "app_id", appID)
	}
	c.syncRunningFlag()
	c.mu.Unlock()
	logger.Info("larkbot: all ws clients stopped")
}

func (c *Client) syncRunningFlag() {
	c.running = false
	for _, e := range c.bots {
		if e != nil && e.wsCancel != nil {
			c.running = true
			return
		}
	}
}

// IsBotWSRunning reports whether this app_id has an active WebSocket session.
func (c *Client) IsBotWSRunning(appID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.bots[appID]
	return ok && e != nil && e.wsCancel != nil
}

func (c *Client) GetBotRuntime(appID string) *agent.Runtime {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[appID]; ok {
		return entry.Runtime
	}
	return nil
}

func (c *Client) GetBotConfig(appID string) *BotConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[appID]; ok {
		return entry.Config
	}
	return nil
}

func (c *Client) GetBots() map[string]*BotEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*BotEntry, len(c.bots))
	for k, v := range c.bots {
		result[k] = v
	}
	return result
}

func (c *Client) GetBotCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.bots)
}

func (c *Client) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *Client) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil {
		return nil
	}

	msg := event.Event.Message
	if msg == nil {
		return nil
	}

	messageID := getStringPtr(msg.MessageId)
	chatID := getStringPtr(msg.ChatId)
	senderID := getSenderIDFromSender(event.Event.Sender)
	senderType := getStringPtr(event.Event.Sender.SenderType)

	logger.Info("larkbot: message received",
		"message_id", messageID,
		"chat_id", chatID,
		"sender_type", senderType,
	)

	if senderType != "user" {
		logger.Info("larkbot: ignoring non-user message")
		return nil
	}

	appID := getAppID(event)
	cfg := c.GetBotConfig(appID)
	if cfg != nil && !c.isSenderAllowed(senderID, cfg.AllowedUsers) {
		logger.Info("larkbot: sender not in allowed_users",
			"sender_id", senderID,
			"app_id", appID,
			"allowed_count", len(cfg.AllowedUsers),
		)
		return nil
	}

	var userInput string
	msgType := getStringPtr(msg.MessageType)
	switch msgType {
	case "text":
		userInput = extractTextFromContent(getStringPtr(msg.Content))
	case "post":
		userInput = extractTextFromPost(getStringPtr(msg.Content))
	}

	if userInput == "" {
		logger.Info("larkbot: empty message, ignoring")
		return nil
	}

	if messageID != "" && c.msgDedup != nil && c.msgDedup.IsDuplicate(messageID) {
		logger.Info("larkbot: duplicate message ignored", "message_id", messageID, "app_id", appID)
		return nil
	}

	// Ack WS quickly; reaction + LLM + reply run async to avoid Lark event redelivery on slow handlers.
	go c.processIncomingMessage(appID, messageID, chatID, senderID, userInput)
	return nil
}

func (c *Client) processIncomingMessage(appID, messageID, chatID, senderID, userInput string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("larkbot: processIncomingMessage panic", "app_id", appID, "recover", r)
		}
	}()
	if !c.IsBotWSRunning(appID) {
		return
	}
	cfg := c.GetBotConfig(appID)
	if cfg != nil && messageID != "" {
		c.addAckReaction(context.Background(), cfg, messageID)
	}

	runtime := c.GetBotRuntime(appID)
	if runtime == nil {
		logger.Error("larkbot: no bot runtime for app_id", "app_id", appID)
		return
	}
	if cfg == nil {
		cfg = c.GetBotConfig(appID)
	}
	if cfg == nil {
		logger.Error("larkbot: no bot config for app_id", "app_id", appID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var sessionID string
	var history []*einoschema.Message
	imUserID := imhistory.FormatIMUserID("lark", senderID)
	if rec := imhistory.GlobalRecorder(); rec != nil && senderID != "" {
		sessionID, history = rec.PrepareConversation(ctx, "lark", cfg.AgentID, senderID, 20)
	}
	if sessionID == "" {
		logger.Warn("larkbot: empty session_id — IM outbound scope NOT set; file attachments cannot stage",
			"agent_id", cfg.AgentID, "sender_id", senderID)
	} else {
		ctx = imoutbound.WithScope(ctx, cfg.AgentID, sessionID)
		logger.Info("larkbot: IM scope set", "agent_id", cfg.AgentID, "session_id", sessionID)
	}

	respText, agentCtx, err := c.invokeAgent(ctx, runtime, cfg, userInput, senderID, sessionID, history)
	if err != nil {
		respText = fmt.Sprintf("处理消息失败: %v", err)
		logger.Error("larkbot: invoke agent failed", "err", err)
	}
	if agentCtx == nil {
		agentCtx = ctx
	}

	scope := imoutbound.Scope{AgentID: cfg.AgentID, SessionID: sessionID}
	store := c.outbound
	if store == nil {
		store = imoutbound.GlobalStore()
	}

	out := imoutbound.DeliverIMReply(imoutbound.DeliverInput{
		Channel: "lark", Ctx: ctx, AgentCtx: agentCtx, Scope: scope, Store: store,
		UserRequest: userInput, AgentText: respText,
	})
	if imoutbound.NeedsFileRetry(userInput, respText, out.MarkerFiles, out.FileNames) {
		logger.Warn("larkbot: IM file staging retry — agent did not register attachments",
			"session_id", sessionID, "markers", out.MarkerFiles,
			"pasted_content", imoutbound.ContainsSalvageableIMPayload(respText),
			"user_input", truncateForLog(userInput, 80))
		retryHistory := append(append([]*einoschema.Message{}, history...),
			&einoschema.Message{Role: einoschema.User, Content: userInput},
			&einoschema.Message{Role: einoschema.Assistant, Content: respText},
		)
		retryPrompt := imoutbound.BuildIMRetryPrompt(userInput, out.MarkerFiles)
		respRetry, agentCtxRetry, retryErr := c.invokeAgent(ctx, runtime, cfg, retryPrompt, senderID, sessionID, retryHistory)
		if retryErr != nil {
			logger.Warn("larkbot: IM file staging retry failed", "err", retryErr)
			if fb := imoutbound.RetryFailureUserText(userInput, "lark", retryErr); fb != "" {
				out.Text = fb
			}
		} else {
			respText = respRetry
			if agentCtxRetry != nil {
				agentCtx = agentCtxRetry
			}
			out = imoutbound.DeliverIMReply(imoutbound.DeliverInput{
				Channel: "lark", Ctx: ctx, AgentCtx: agentCtx, Scope: scope, Store: store,
				UserRequest: userInput, AgentText: respText,
			})
			logger.Info("larkbot: IM file staging retry done", "staged", imoutbound.RegisteredFilesFromContext(agentCtx), "to_send", out.FileNames)
		}
	}

	target := MessageTarget{
		ReplyMessageID: messageID,
		ChatID:         chatID,
		OpenID:         senderID,
	}

	if target.canReplyText() && strings.TrimSpace(out.Text) != "" {
		if err := c.replyMessage(ctx, cfg, target.ReplyMessageID, out.Text); err != nil {
			logger.Error("larkbot: reply send failed", "message_id", messageID, "err", err)
		}
	}
	if len(out.FileNames) > 0 {
		if target.canSendAttachment() {
			c.sendOutboundFiles(ctx, cfg, target, scope, out.FileNames)
		} else {
			logger.Error("larkbot: have files to send but no chat_id/open_id", "files", out.FileNames)
		}
	}

	if rec := imhistory.GlobalRecorder(); rec != nil && sessionID != "" {
		rec.RecordTurnAsync(cfg.AgentID, sessionID, imUserID, userInput, respText)
	}
}

func (c *Client) invokeAgent(ctx context.Context, runtime *agent.Runtime, cfg *BotConfig, userInput, larkUserID, sessionID string, history []*einoschema.Message) (string, context.Context, error) {
	timeout := cfg.InvokeTimeout
	if timeout <= 0 {
		timeout = 120
	}
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	memContext := imoutbound.IMMemoryContextHint()
	resp, err := runtime.ChatWithMemoryContext(agentCtx, cfg.AgentID, userInput, "", "", memContext, sessionID, larkUserID, history)
	return resp, agentCtx, err
}

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func getStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getSenderIDFromSender(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}
	if sender.SenderId.OpenId != nil {
		return *sender.SenderId.OpenId
	}
	if sender.SenderId.UnionId != nil {
		return *sender.SenderId.UnionId
	}
	if sender.SenderId.UserId != nil {
		return *sender.SenderId.UserId
	}
	return ""
}

func extractTextFromContent(content string) string {
	if content == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return content
	}
	if t, ok := m["text"].(string); ok {
		return t
	}
	return content
}

func extractTextFromPost(content string) string {
	if content == "" {
		return ""
	}
	return content
}

func getAppID(event *larkim.P2MessageReceiveV1) string {
	if event == nil {
		return ""
	}
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		return event.EventV2Base.Header.AppID
	}
	return ""
}

func (c *Client) isSenderAllowed(senderOpenID string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	senderOpenID = strings.TrimSpace(senderOpenID)
	if senderOpenID == "" {
		return false
	}
	for _, raw := range allowed {
		if strings.TrimSpace(raw) == senderOpenID {
			return true
		}
	}
	return false
}
