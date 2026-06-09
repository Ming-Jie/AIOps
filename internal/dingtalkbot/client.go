package dingtalkbot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	einoschema "github.com/cloudwego/eino/schema"
	dtchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dtclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/imhistory"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/larksuite/oapi-sdk-go/v3/channel/safety"
)

const (
	conversationTypePrivate = "1"
	conversationTypeGroup   = "2"
)

const (
	// coalesceWindow buffers rapid consecutive messages from the same conversation
	// into a single agent turn, so quick follow-ups are answered together.
	coalesceWindow = 1200 * time.Millisecond
	// processingNoticeDelay sends a "working on it" reply only when a turn runs
	// longer than this, so fast replies stay quiet while slow ones get feedback.
	processingNoticeDelay = 3 * time.Second
	processingNoticeText  = "🤔 正在处理你的消息，请稍候…"
)

// pendingDtMsg is a validated inbound message awaiting batched processing.
type pendingDtMsg struct {
	userInput      string
	sessionWebhook string
	senderID       string
}

// convoState serializes and coalesces messages for one conversation.
type convoState struct {
	pending    []*pendingDtMsg
	timer      *time.Timer
	processing bool
}

type BotConfig struct {
	AppID          string
	AppSecret      string
	AgentID        int64
	InvokeTimeout  int
	NoAutoStartWS  bool
	AllowedUsers   []string
	RequireMention bool
}

type BotEntry struct {
	Config       *BotConfig
	Runtime      *agent.Runtime
	streamClient *dtclient.StreamClient
	streamCancel context.CancelFunc
	streamGen    uint64
	running      bool
}

type Client struct {
	mu       sync.RWMutex
	bots     map[string]*BotEntry
	running  bool
	msgDedup *safety.DedupCache
	tokens   *tokenManager
	outbound *imoutbound.Store

	convoMu sync.Mutex
	convos  map[string]*convoState
}

var globalClient *Client
var streamGenSeq uint64

func Global() *Client {
	if globalClient == nil {
		globalClient = NewClient()
	}
	return globalClient
}

func NewClient() *Client {
	return &Client{
		bots:     make(map[string]*BotEntry),
		msgDedup: safety.NewDedupCache(10000, time.Hour),
		tokens:   newTokenManager(),
		convos:   make(map[string]*convoState),
	}
}

func (c *Client) RegisterBot(cfg *BotConfig, runtime *agent.Runtime) error {
	if cfg == nil || cfg.AppID == "" || cfg.AppSecret == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: app_id, app_secret and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.bots[cfg.AppID]; exists {
		if existing.Config.AgentID != cfg.AgentID {
			logger.Warn("dingtalkbot: app_id already bound to another agent",
				"app_id", cfg.AppID, "new_agent_id", cfg.AgentID, "existing_agent_id", existing.Config.AgentID)
			return fmt.Errorf("app_id already bound to agent %d", existing.Config.AgentID)
		}
		existing.Config = cfg
		existing.Runtime = runtime
		logger.Info("dingtalkbot: bot re-registered", "agent_id", cfg.AgentID, "app_id", cfg.AppID)
		return nil
	}

	c.bots[cfg.AppID] = &BotEntry{
		Config:  cfg,
		Runtime: runtime,
	}
	logger.Info("dingtalkbot: bot registered", "agent_id", cfg.AgentID, "app_id", cfg.AppID)
	return nil
}

func (c *Client) UnregisterBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[appID]
	if ok {
		c.stopEntryLocked(entry)
		delete(c.bots, appID)
		logger.Info("dingtalkbot: bot unregistered", "app_id", appID)
	}
	c.syncRunningFlag()
}

func (c *Client) UpdateBotConfig(cfg *BotConfig) error {
	if cfg == nil || cfg.AppID == "" || cfg.AppSecret == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: app_id, app_secret and agent_id required")
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
	logger.Info("dingtalkbot: bot config updated", "agent_id", cfg.AgentID, "app_id", cfg.AppID)
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

func (c *Client) GetBotRuntime(appID string) *agent.Runtime {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if entry, ok := c.bots[appID]; ok {
		return entry.Runtime
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

func (c *Client) IsBotRunning(appID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.bots[appID]
	return ok && entry != nil && entry.running
}

func (c *Client) Start(ctx context.Context) {
	c.mu.Lock()
	appIDs := make([]string, 0, len(c.bots))
	for id, e := range c.bots {
		if e != nil && e.Config != nil && e.Config.NoAutoStartWS {
			logger.Info("dingtalkbot: skip global start (ws_enabled false)", "app_id", id)
			continue
		}
		appIDs = append(appIDs, id)
	}
	c.mu.Unlock()

	if len(appIDs) == 0 {
		if c.GetBotCount() == 0 {
			logger.Info("dingtalkbot: no bots registered, skip starting")
		} else {
			logger.Info("dingtalkbot: no bots eligible for global start (all ws_enabled false)")
		}
		return
	}

	for _, appID := range appIDs {
		if err := c.StartBot(ctx, appID); err != nil {
			logger.Warn("dingtalkbot: start bot skipped", "app_id", appID, "err", err)
		}
	}
	logger.Info("dingtalkbot: global start complete", "started", len(appIDs))
}

func (c *Client) StartBot(parentCtx context.Context, appID string) error {
	c.mu.Lock()
	entry, ok := c.bots[appID]
	if !ok || entry == nil {
		c.mu.Unlock()
		return fmt.Errorf("bot not found for app_id: %s", appID)
	}
	if entry.running {
		c.mu.Unlock()
		return nil
	}
	cfg := entry.Config
	c.mu.Unlock()

	streamClient := dtclient.NewStreamClient(
		dtclient.WithAppCredential(dtclient.NewAppCredentialConfig(cfg.AppID, cfg.AppSecret)),
		dtclient.WithAutoReconnect(true),
	)
	streamClient.RegisterChatBotCallbackRouter(c.makeChatHandler(appID))

	ctx, cancel := context.WithCancel(parentCtx)
	gen := atomic.AddUint64(&streamGenSeq, 1)

	c.mu.Lock()
	entry, ok = c.bots[appID]
	if !ok || entry == nil {
		c.mu.Unlock()
		cancel()
		return fmt.Errorf("bot not found for app_id: %s", appID)
	}
	entry.streamClient = streamClient
	entry.streamCancel = cancel
	entry.streamGen = gen
	entry.running = true
	c.syncRunningFlag()
	c.mu.Unlock()

	go func() {
		logger.Info("dingtalkbot: stream client starting", "app_id", appID)
		if err := streamClient.Start(ctx); err != nil {
			if ctx.Err() == nil {
				logger.Error("dingtalkbot: stream client start failed", "app_id", appID, "err", err)
			}
			c.mu.Lock()
			if e, ok2 := c.bots[appID]; ok2 && e.streamGen == gen {
				c.stopEntryLocked(e)
			}
			c.syncRunningFlag()
			c.mu.Unlock()
			return
		}
		<-ctx.Done()
		streamClient.AutoReconnect = false
		streamClient.Close()
		c.mu.Lock()
		if e, ok2 := c.bots[appID]; ok2 && e.streamGen == gen {
			e.streamClient = nil
			e.streamCancel = nil
			e.running = false
		}
		c.syncRunningFlag()
		c.mu.Unlock()
		logger.Info("dingtalkbot: stream client stopped", "app_id", appID)
	}()

	return nil
}

func (c *Client) StopBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.bots[appID]
	if !ok || entry == nil {
		return
	}
	c.stopEntryLocked(entry)
	c.syncRunningFlag()
	logger.Info("dingtalkbot: stream client stopped for app", "app_id", appID)
}

func (c *Client) Stop() {
	c.mu.Lock()
	for appID, entry := range c.bots {
		c.stopEntryLocked(entry)
		logger.Info("dingtalkbot: stream client stopped for app", "app_id", appID)
	}
	c.syncRunningFlag()
	c.mu.Unlock()
	logger.Info("dingtalkbot: all stream clients stopped")
}

func (c *Client) stopEntryLocked(entry *BotEntry) {
	if entry == nil {
		return
	}
	if entry.streamClient != nil {
		entry.streamClient.AutoReconnect = false
		entry.streamClient.Close()
		entry.streamClient = nil
	}
	if entry.streamCancel != nil {
		entry.streamCancel()
		entry.streamCancel = nil
	}
	entry.running = false
}

func (c *Client) syncRunningFlag() {
	c.running = false
	for _, e := range c.bots {
		if e != nil && e.running {
			c.running = true
			return
		}
	}
}

func (c *Client) makeChatHandler(appID string) dtchatbot.IChatBotMessageHandler {
	return func(ctx context.Context, data *dtchatbot.BotCallbackDataModel) ([]byte, error) {
		if data == nil {
			return []byte(""), nil
		}
		go c.processIncomingMessage(appID, data)
		return []byte(""), nil
	}
}

func (c *Client) processIncomingMessage(appID string, data *dtchatbot.BotCallbackDataModel) {
	if !c.IsBotRunning(appID) {
		return
	}
	cfg := c.GetBotConfig(appID)
	if cfg == nil {
		logger.Warn("dingtalkbot: no bot config", "app_id", appID)
		return
	}

	senderID := strings.TrimSpace(data.SenderStaffId)
	if senderID == "" {
		senderID = strings.TrimSpace(data.SenderId)
	}
	if !isSenderAllowed(senderID, cfg.AllowedUsers) {
		logger.Info("dingtalkbot: sender not in allowed_users", "sender_id", senderID, "app_id", appID)
		return
	}

	if data.ConversationType == conversationTypeGroup && cfg.RequireMention && !data.IsInAtList {
		logger.Info("dingtalkbot: group message without @mention, ignoring", "app_id", appID)
		return
	}

	userInput := extractText(data)
	if userInput == "" {
		logger.Info("dingtalkbot: empty message, ignoring", "app_id", appID)
		return
	}

	dedupKey := strings.TrimSpace(data.MsgId)
	if dedupKey == "" {
		dedupKey = fmt.Sprintf("%s:%d", appID, data.CreateAt)
	}
	if c.msgDedup != nil && c.msgDedup.IsDuplicate(dedupKey) {
		logger.Info("dingtalkbot: duplicate message ignored", "msg_id", data.MsgId, "app_id", appID)
		return
	}

	runtime := c.GetBotRuntime(appID)
	if runtime == nil {
		logger.Error("dingtalkbot: no bot runtime", "app_id", appID)
		return
	}

	sessionWebhook := strings.TrimSpace(data.SessionWebhook)
	if sessionWebhook == "" {
		logger.Warn("dingtalkbot: missing session webhook", "app_id", appID)
		return
	}

	msgID := strings.TrimSpace(data.MsgId)
	conversationID := strings.TrimSpace(data.ConversationId)
	if cfg != nil && msgID != "" && conversationID != "" {
		go c.addAckEmotion(context.Background(), cfg, msgID, conversationID)
	}

	key := conversationID + "|" + senderID
	c.enqueueMessage(appID, key, &pendingDtMsg{
		userInput:      userInput,
		sessionWebhook: sessionWebhook,
		senderID:       senderID,
	})
}

// enqueueMessage buffers a message into its conversation's queue. Rapid messages
// within coalesceWindow are merged into one turn; while a turn is running, new
// messages wait and are drained as the next turn (no parallel slow invocations).
func (c *Client) enqueueMessage(appID, key string, m *pendingDtMsg) {
	c.convoMu.Lock()
	defer c.convoMu.Unlock()

	st := c.convos[key]
	if st == nil {
		st = &convoState{}
		c.convos[key] = st
	}
	st.pending = append(st.pending, m)
	if st.processing || st.timer != nil {
		return
	}
	st.timer = time.AfterFunc(coalesceWindow, func() { c.runConversationBatch(appID, key) })
}

// runConversationBatch drains the conversation's buffered messages and processes
// them as a single turn, then re-schedules if more arrived during processing.
func (c *Client) runConversationBatch(appID, key string) {
	c.convoMu.Lock()
	st := c.convos[key]
	if st == nil {
		c.convoMu.Unlock()
		return
	}
	st.timer = nil
	if st.processing || len(st.pending) == 0 {
		c.convoMu.Unlock()
		return
	}
	batch := st.pending
	st.pending = nil
	st.processing = true
	c.convoMu.Unlock()

	c.handleBatch(appID, batch)

	c.convoMu.Lock()
	st = c.convos[key]
	if st == nil {
		c.convoMu.Unlock()
		return
	}
	st.processing = false
	if len(st.pending) > 0 {
		if st.timer == nil {
			st.timer = time.AfterFunc(0, func() { c.runConversationBatch(appID, key) })
		}
	} else {
		delete(c.convos, key)
	}
	c.convoMu.Unlock()
}

// handleBatch runs the agent once for a coalesced batch and replies via the
// freshest sessionWebhook (least likely to have expired during a slow turn).
func (c *Client) handleBatch(appID string, batch []*pendingDtMsg) {
	cfg := c.GetBotConfig(appID)
	runtime := c.GetBotRuntime(appID)
	if cfg == nil || runtime == nil || len(batch) == 0 {
		return
	}

	last := batch[len(batch)-1]
	sessionWebhook := last.sessionWebhook
	senderID := last.senderID

	parts := make([]string, 0, len(batch))
	for _, m := range batch {
		if s := strings.TrimSpace(m.userInput); s != "" {
			parts = append(parts, s)
		}
	}
	userInput := strings.Join(parts, "\n")
	if userInput == "" {
		return
	}

	ctx := context.Background()
	var sessionID string
	var history []*einoschema.Message
	imUserID := imhistory.FormatIMUserID("dingtalk", senderID)
	if rec := imhistory.GlobalRecorder(); rec != nil && senderID != "" {
		sessionID, history = rec.PrepareConversation(ctx, "dingtalk", cfg.AgentID, senderID, 20)
	}
	if sessionID != "" {
		ctx = imoutbound.WithScope(ctx, cfg.AgentID, sessionID)
	}

	timeout := cfg.InvokeTimeout
	if timeout <= 0 {
		timeout = 120
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Send a "working on it" notice only if the turn is slow; cancelled if the
	// reply lands first.
	notice := time.AfterFunc(processingNoticeDelay, func() {
		nctx, ncancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ncancel()
		if err := c.replySessionMessageOnce(nctx, cfg, sessionWebhook, processingNoticeText, 1, 1); err != nil {
			logger.Warn("dingtalkbot: send processing notice failed", "err", err, "app_id", appID)
		}
	})

	respText, err := c.invokeAgent(ctx, runtime, cfg, userInput, senderID, sessionID, history)
	notice.Stop()
	if err != nil {
		respText = fmt.Sprintf("处理消息失败: %v", err)
		logger.Error("dingtalkbot: invoke agent failed", "err", err, "app_id", appID)
	}

	scope := imoutbound.Scope{AgentID: cfg.AgentID, SessionID: sessionID}
	replyText, fileNames := imoutbound.ParseFileMarkers(respText)

	replyCtx, replyCancel := replyContext(ctx)
	defer replyCancel()

	if strings.TrimSpace(replyText) != "" {
		if err := c.replySessionMessage(replyCtx, cfg, sessionWebhook, replyText); err != nil {
			logger.Warn("dingtalkbot: send reply failed", "err", err, "app_id", appID)
		}
	}
	if len(fileNames) > 0 {
		failed := c.sendOutboundFiles(replyCtx, cfg, sessionWebhook, scope, fileNames)
		if len(failed) > 0 {
			hint := fmt.Sprintf("⚠️ 以下附件发送失败：%s", strings.Join(failed, ", "))
			_ = c.replySessionMessage(replyCtx, cfg, sessionWebhook, hint)
		}
	}

	if rec := imhistory.GlobalRecorder(); rec != nil && sessionID != "" {
		rec.RecordTurnAsync(cfg.AgentID, sessionID, imUserID, userInput, respText)
	}
}

func (c *Client) invokeAgent(ctx context.Context, runtime *agent.Runtime, cfg *BotConfig, userInput, auditUserID, sessionID string, history []*einoschema.Message) (string, error) {
	memContext := dingtalkFileSendSystemHint()
	return runtime.ChatWithMemoryContext(ctx, cfg.AgentID, userInput, "", "", memContext, sessionID, auditUserID, history)
}

func extractText(data *dtchatbot.BotCallbackDataModel) string {
	if data == nil {
		return ""
	}
	if text := strings.TrimSpace(data.Text.Content); text != "" {
		return text
	}
	if s, ok := data.Content.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func isSenderAllowed(senderID string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	senderID = strings.TrimSpace(senderID)
	if senderID == "" {
		return false
	}
	for _, u := range allowed {
		if strings.TrimSpace(u) == senderID {
			return true
		}
	}
	return false
}

