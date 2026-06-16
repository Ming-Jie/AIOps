package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/imhistory"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/gorilla/websocket"
	"github.com/larksuite/oapi-sdk-go/v3/channel/safety"
)

const (
	qqAPIBase         = "https://api.sgroup.qq.com"
	qqSandboxBase     = "https://sandbox.api.sgroup.qq.com"
	qqWSURL           = "wss://api.sgroup.qq.com/websocket/"
	qqSandboxWSURL    = "wss://sandbox.api.sgroup.qq.com/websocket/"
	qqOAuthURL        = "https://bots.qq.com/app/getAppAccessToken"
	qqOAuthSandboxURL = "https://sandbox.bots.qq.com/app/getAppAccessToken"
)

type wsOpCode int

const (
	opDispatch       wsOpCode = 0
	opHeartbeat      wsOpCode = 1
	opIdentify       wsOpCode = 2
	opResume         wsOpCode = 6
	opReconnect      wsOpCode = 7
	opInvalidSession wsOpCode = 9
	opHello          wsOpCode = 10
	opHeartbeatAck   wsOpCode = 11
)

type BotConfig struct {
	AppID         string
	BotToken      string // AppSecret, used with AppID to obtain access_token via OAuth
	AgentID       int64
	InvokeTimeout int
	NoAutoStartWS bool
	Sandbox       bool
	AllowedUsers  []string
}

type BotEntry struct {
	Config    *BotConfig
	Runtime   *agent.Runtime
	wsConn    *websocket.Conn
	wsCancel  context.CancelFunc
	wsSession uint64

	tokenMu        sync.Mutex
	accessToken    string
	tokenExpiresAt time.Time
}

var wsSessionSeq uint64

type Client struct {
	mu       sync.RWMutex
	bots     map[string]*BotEntry
	running  bool
	msgDedup *safety.DedupCache
}

var globalClient *Client

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
	}
}

// ---- OAuth2 Access Token ----

type oauthTokenRequest struct {
	AppID        string `json:"appId"`
	ClientSecret string `json:"clientSecret"`
}

type oauthTokenResponse struct {
	AccessToken string          `json:"access_token"`
	ExpiresIn   json.RawMessage `json:"expires_in"`
}

func oauthURL(sandbox bool) string {
	if sandbox {
		return qqOAuthSandboxURL
	}
	return qqOAuthURL
}

func (e *BotEntry) fetchAccessToken(ctx context.Context) error {
	reqBody := oauthTokenRequest{
		AppID:        e.Config.AppID,
		ClientSecret: e.Config.BotToken,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal oauth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthURL(e.Config.Sandbox), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create oauth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("oauth http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("oauth status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var tokenResp oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decode oauth response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("oauth response missing access_token")
	}

	expiresIn := 7200
	if len(tokenResp.ExpiresIn) > 0 {
		// try int first, then string
		if err := json.Unmarshal(tokenResp.ExpiresIn, &expiresIn); err != nil {
			var s string
			if err2 := json.Unmarshal(tokenResp.ExpiresIn, &s); err2 == nil {
				fmt.Sscanf(s, "%d", &expiresIn)
			}
		}
	}
	if expiresIn <= 0 {
		expiresIn = 7200
	}

	e.tokenMu.Lock()
	e.accessToken = tokenResp.AccessToken
	// Refresh 60 seconds before expiry
	e.tokenExpiresAt = time.Now().Add(time.Duration(expiresIn-60) * time.Second)
	e.tokenMu.Unlock()

	logger.Info("qqbot: access token obtained", "app_id", e.Config.AppID,
		"expires_in", expiresIn, "refresh_at", e.tokenExpiresAt)
	return nil
}

func (e *BotEntry) ensureToken(ctx context.Context) error {
	e.tokenMu.Lock()
	needRefresh := e.accessToken == "" || time.Now().After(e.tokenExpiresAt)
	e.tokenMu.Unlock()

	if !needRefresh {
		return nil
	}
	return e.fetchAccessToken(ctx)
}

func (e *BotEntry) authHeader() string {
	e.tokenMu.Lock()
	token := e.accessToken
	e.tokenMu.Unlock()
	if token == "" {
		return ""
	}
	return "QQBot " + token
}

// ---- URL helpers ----

func apiBase(sandbox bool) string {
	if sandbox {
		return qqSandboxBase
	}
	return qqAPIBase
}

func wsURL(sandbox bool) string {
	if sandbox {
		return qqSandboxWSURL
	}
	return qqWSURL
}

// ---- Bot entry management ----

func (c *Client) RegisterBot(cfg *BotConfig, runtime *agent.Runtime) error {
	if cfg == nil || cfg.AppID == "" || cfg.BotToken == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: app_id, bot_token and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.bots[cfg.AppID]; exists {
		if existing.Config.AgentID != cfg.AgentID {
			logger.Warn("qqbot: app_id already bound to another agent",
				"app_id", cfg.AppID, "new_agent_id", cfg.AgentID, "existing_agent_id", existing.Config.AgentID)
			return fmt.Errorf("app_id %s already bound to agent %d", cfg.AppID, existing.Config.AgentID)
		}
		// Invalidate cached token on config update
		existing.tokenMu.Lock()
		existing.accessToken = ""
		existing.tokenMu.Unlock()
		existing.Config = cfg
		existing.Runtime = runtime
		logger.Info("qqbot: bot re-registered", "agent_id", cfg.AgentID, "app_id", cfg.AppID)
		return nil
	}

	c.bots[cfg.AppID] = &BotEntry{
		Config:  cfg,
		Runtime: runtime,
	}

	logger.Info("qqbot: bot registered", "agent_id", cfg.AgentID, "app_id", cfg.AppID)
	return nil
}

func (c *Client) UnregisterBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[appID]
	if ok {
		closeBotWSEntry(entry)
		delete(c.bots, appID)
		logger.Info("qqbot: bot unregistered (ws client closed)", "app_id", appID)
	}
}

func (c *Client) UpdateBotConfig(cfg *BotConfig) error {
	if cfg == nil || cfg.AppID == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config")
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

	// Invalidate cached token on config update
	entry.tokenMu.Lock()
	entry.accessToken = ""
	entry.tokenMu.Unlock()

	entry.Config = cfg
	logger.Info("qqbot: bot config updated", "agent_id", cfg.AgentID, "app_id", cfg.AppID)
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

func (c *Client) IsBotWSRunning(appID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.bots[appID]
	return ok && entry != nil && entry.wsCancel != nil
}

// ---- WebSocket lifecycle ----

func (c *Client) appIDsForGlobalStart() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var ids []string
	for id, e := range c.bots {
		if e != nil && e.Config != nil && e.Config.NoAutoStartWS {
			logger.Info("qqbot: skip global start (ws_enabled false)", "app_id", id)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func (c *Client) Start(ctx context.Context) {
	ids := c.appIDsForGlobalStart()
	if len(ids) == 0 {
		if c.GetBotCount() == 0 {
			logger.Info("qqbot: no bots registered, skip starting")
		} else {
			logger.Info("qqbot: no bots eligible for global start (all ws_enabled false)")
		}
		return
	}

	for _, appID := range ids {
		if err := c.StartBot(ctx, appID); err != nil {
			logger.Warn("qqbot: start bot skipped", "app_id", appID, "err", err)
		}
	}
	logger.Info("qqbot: all eligible ws clients started", "bot_count", len(ids))
}

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

	// Get access token before starting WS
	if err := entry.ensureToken(parentCtx); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("get access token: %w", err)
	}

	cfg := entry.Config
	ctx, cancel := context.WithCancel(parentCtx)
	sess := atomic.AddUint64(&wsSessionSeq, 1)
	entry.wsCancel = cancel
	entry.wsSession = sess
	c.syncRunningFlag()
	c.mu.Unlock()

	go func() {
		logger.Info("qqbot: ws client starting", "app_id", appID, "sandbox", cfg.Sandbox)

		const maxBackoff = 30 * time.Second
		backoff := 1 * time.Second

		for {
			err := c.connectWS(ctx, entry, appID, sess)
			if ctx.Err() != nil {
				break
			}

			logger.Warn("qqbot: ws disconnected, reconnecting",
				"app_id", appID, "err", err, "backoff", backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				break
			}
			if ctx.Err() != nil {
				break
			}

			if err := entry.ensureToken(ctx); err != nil {
				logger.Warn("qqbot: token refresh before reconnect failed", "app_id", appID, "err", err)
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		c.mu.Lock()
		if e, ok2 := c.bots[appID]; ok2 && e.wsSession == sess {
			e.wsConn = nil
			e.wsCancel = nil
		}
		c.syncRunningFlag()
		c.mu.Unlock()

		if ctx.Err() == nil {
			logger.Info("qqbot: ws client reconnect stopped", "app_id", appID)
		}
	}()

	return nil
}

// ---- Wire protocol types ----

type wsPayload struct {
	Op   wsOpCode        `json:"op"`
	D    json.RawMessage `json:"d,omitempty"`
	S    *int64          `json:"s,omitempty"`
	T    string          `json:"t,omitempty"`
	ID   string          `json:"id,omitempty"`
}

type identifyData struct {
	Token      string      `json:"token"`
	Intents    int         `json:"intents"`
	Shard      []int       `json:"shard,omitempty"`
	Properties *properties `json:"properties,omitempty"`
}

type properties struct {
	OS      string `json:"$os,omitempty"`
	Browser string `json:"$browser,omitempty"`
	Device  string `json:"$device,omitempty"`
}

type helloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type readyData struct {
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	User      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"user"`
	Shard []int `json:"shard"`
}

// ---- Event data structures (official QQ Bot v2 format) ----

type c2cEventAuthor struct {
	UserOpenID string `json:"user_openid"`
}

type c2cEventBody struct {
	Author    c2cEventAuthor `json:"author"`
	Content   string         `json:"content"`
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
}

type groupEventAuthor struct {
	MemberOpenID string `json:"member_openid"`
}

type groupEventBody struct {
	Author      groupEventAuthor `json:"author"`
	Content     string           `json:"content"`
	GroupOpenID string           `json:"group_openid"`
	ID          string           `json:"id"`
	Timestamp   string           `json:"timestamp"`
}

// ---- Group lifecycle events (also intents 1<<25) ----

type groupLifecycleEvent struct {
	GroupOpenID    string `json:"group_openid"`
	OpMemberOpenID string `json:"op_member_openid"`
	Timestamp      int64  `json:"timestamp"`
}

type friendEvent struct {
	OpenID    string `json:"openid"`
	Timestamp int64  `json:"timestamp"`
}

// ---- WebSocket connection loop ----

func (c *Client) connectWS(ctx context.Context, entry *BotEntry, appID string, sess uint64) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := make(http.Header)
	header.Set("Authorization", entry.authHeader())

	cfg := entry.Config
	url := wsURL(cfg.Sandbox)
	conn, _, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		return fmt.Errorf("dial ws %s: %w", url, err)
	}
	defer conn.Close()

	key := appID
	c.mu.Lock()
	if e, ok := c.bots[key]; ok && e.wsSession == sess {
		e.wsConn = conn
	}
	c.mu.Unlock()

	logger.Info("qqbot: ws connected", "app_id", appID, "url", url)

	// Read Hello (OpCode 10)
	var hello wsPayload
	if err := readJSON(conn, &hello); err != nil {
		return fmt.Errorf("read hello: %w", err)
	}
	if hello.Op != opHello {
		return fmt.Errorf("unexpected first op %d, expected hello (10)", hello.Op)
	}
	var hd helloData
	if hello.D != nil {
		json.Unmarshal(hello.D, &hd)
	}
	heartbeatInterval := hd.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30000
	}
	logger.Info("qqbot: hello received", "app_id", appID, "heartbeat_interval", heartbeatInterval)

	// Send Identify (OpCode 2) with access token in QQBot format
	idData := identifyData{
		Token:   "QQBot " + entry.accessToken,
		Intents: 1 << 25, // GROUP_AND_C2C_EVENT
		Shard:   []int{0, 1},
	}
	sendPayload(conn, opIdentify, idData)

	// Start heartbeat ticker
	heartbeat := time.NewTicker(time.Duration(heartbeatInterval) * time.Millisecond)
	defer heartbeat.Stop()

	// Read and dispatch goroutine
	type wsMessage struct {
		data []byte
		err  error
	}
	readCh := make(chan wsMessage, 128)
	go func() {
		defer close(readCh)
		for {
			_, message, err := conn.ReadMessage()
			select {
			case readCh <- wsMessage{data: message, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	var seq int64
	var sessionID string
	identified := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			var hbPayload []byte
			if seq > 0 {
				hbPayload, _ = json.Marshal(wsPayload{Op: opHeartbeat, D: mustMarshal(seq)})
			} else {
				hbPayload, _ = json.Marshal(wsPayload{Op: opHeartbeat})
			}
			conn.WriteMessage(websocket.TextMessage, hbPayload)
		case msg, ok := <-readCh:
			if !ok {
				return fmt.Errorf("ws read channel closed")
			}
			if msg.err != nil {
				return fmt.Errorf("ws read error: %w", msg.err)
			}
			var p wsPayload
			if err := json.Unmarshal(msg.data, &p); err != nil {
				continue
			}
			switch p.Op {
			case opDispatch:
				if p.S != nil {
					seq = *p.S
				}
				switch p.T {
				case "READY":
					var rd readyData
					if p.D != nil {
						json.Unmarshal(p.D, &rd)
					}
					sessionID = rd.SessionID
					identified = true
					logger.Info("qqbot: ready event received",
						"app_id", appID, "session_id", sessionID,
						"bot_id", rd.User.ID, "bot_name", rd.User.Username)
				case "RESUMED":
					identified = true
					logger.Info("qqbot: resumed event received", "app_id", appID)
				case "C2C_MESSAGE_CREATE":
					if !identified {
						continue
					}
					c.handleC2CMessage(cfg, appID, p.D, p.ID)
				case "GROUP_AT_MESSAGE_CREATE":
					if !identified {
						continue
					}
					c.handleGroupMessage(cfg, appID, p.D, p.ID)
				case "GROUP_ADD_ROBOT":
					c.handleGroupLifecycle("added", appID, p.D)
				case "GROUP_DEL_ROBOT":
					c.handleGroupLifecycle("removed", appID, p.D)
				case "GROUP_MSG_REJECT":
					c.handleGroupLifecycle("msg_reject", appID, p.D)
				case "GROUP_MSG_RECEIVE":
					c.handleGroupLifecycle("msg_receive", appID, p.D)
				case "FRIEND_ADD":
					c.handleFriendEvent("added", appID, p.D)
				case "FRIEND_DEL":
					c.handleFriendEvent("removed", appID, p.D)
				default:
					logger.Debug("qqbot: unhandled event", "app_id", appID, "t", p.T)
				}
			case opHeartbeatAck:
			case opReconnect:
				logger.Info("qqbot: reconnect requested", "app_id", appID)
				return fmt.Errorf("reconnect requested")
			case opInvalidSession:
				logger.Warn("qqbot: invalid session", "app_id", appID)
				return fmt.Errorf("invalid session")
			}
		}
	}
}

func readJSON(conn *websocket.Conn, v any) error {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(msg, v)
}

func sendPayload(conn *websocket.Conn, op wsOpCode, d any) error {
	b, _ := json.Marshal(wsPayload{Op: op, D: mustMarshal(d)})
	return conn.WriteMessage(websocket.TextMessage, b)
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// ---- Lifecycle event handlers ----

func (c *Client) handleGroupLifecycle(action, appID string, data json.RawMessage) {
	var ev groupLifecycleEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return
	}
	logger.Info("qqbot: group lifecycle event",
		"app_id", appID, "action", action,
		"group_openid", ev.GroupOpenID,
		"op_member_openid", ev.OpMemberOpenID)
}

func (c *Client) handleFriendEvent(action, appID string, data json.RawMessage) {
	var ev friendEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return
	}
	logger.Info("qqbot: friend event",
		"app_id", appID, "action", action,
		"openid", ev.OpenID)
}

// ---- Message event handlers ----

func (c *Client) handleC2CMessage(cfg *BotConfig, appID string, data json.RawMessage, eventID string) {
	var body c2cEventBody
	if err := json.Unmarshal(data, &body); err != nil {
		return
	}
	userInput := strings.TrimSpace(body.Content)
	if userInput == "" {
		return
	}

	dedupKey := fmt.Sprintf("c2c:%s:%s", appID, body.ID)
	if c.msgDedup != nil && c.msgDedup.IsDuplicate(dedupKey) {
		return
	}

	openID := body.Author.UserOpenID
	if openID == "" {
		return
	}

	go c.processIncomingMessage(appID, openID, "", userInput, "private", body.ID)
}

func (c *Client) handleGroupMessage(cfg *BotConfig, appID string, data json.RawMessage, eventID string) {
	var body groupEventBody
	if err := json.Unmarshal(data, &body); err != nil {
		return
	}
	userInput := strings.TrimSpace(body.Content)
	if userInput == "" {
		return
	}

	dedupKey := fmt.Sprintf("group:%s:%s", appID, body.ID)
	if c.msgDedup != nil && c.msgDedup.IsDuplicate(dedupKey) {
		return
	}

	memberOpenID := body.Author.MemberOpenID
	if memberOpenID == "" {
		return
	}
	groupOpenID := body.GroupOpenID
	if groupOpenID == "" {
		return
	}

	go c.processIncomingMessage(appID, memberOpenID, groupOpenID, userInput, "group", body.ID)
}

// ---- Message processing ----

func (c *Client) processIncomingMessage(appID, senderID, groupOpenID, userInput, msgType, msgID string) {
	if !c.IsBotWSRunning(appID) {
		return
	}

	cfg := c.GetBotConfig(appID)
	if cfg == nil {
		logger.Error("qqbot: no bot config for app_id", "app_id", appID)
		return
	}

	if !isSenderAllowed(senderID, cfg.AllowedUsers) {
		logger.Info("qqbot: sender not in allowed_users", "sender_id", senderID, "app_id", appID)
		return
	}

	runtime := c.GetBotRuntime(appID)
	if runtime == nil {
		logger.Error("qqbot: no bot runtime for app_id", "app_id", appID)
		return
	}

	ctx := context.Background()

	var sessionID string
	var history []*einoschema.Message
	imUserID := imhistory.FormatIMUserID("qq", senderID)
	if rec := imhistory.GlobalRecorder(); rec != nil && senderID != "" {
		sessionID, history = rec.PrepareConversation(ctx, "qq", cfg.AgentID, senderID, 20)
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

	respText, err := runtime.ChatWithMemoryContext(ctx, cfg.AgentID, userInput, "", "", imoutbound.IMMemoryContextHint(), sessionID, imUserID, history)
	if err != nil {
		respText = fmt.Sprintf("处理消息失败: %v", err)
		logger.Error("qqbot: invoke agent failed", "err", err)
	}

	scope := imoutbound.Scope{AgentID: cfg.AgentID, SessionID: sessionID}
	out := imoutbound.DeliverIMReply(imoutbound.DeliverInput{
		Channel: "qq", Ctx: ctx, AgentCtx: ctx, Scope: scope, Store: imoutbound.GlobalStore(),
		UserRequest: userInput, AgentText: respText,
	})

	replyCtx, replyCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer replyCancel()

	if strings.TrimSpace(out.Text) != "" {
		if msgType == "group" && groupOpenID != "" {
			_ = c.sendGroupMsg(replyCtx, appID, groupOpenID, out.Text, msgID)
		} else {
			_ = c.sendPrivateMsg(replyCtx, appID, senderID, out.Text, msgID)
		}
	}

	if len(out.FileNames) > 0 {
		c.sendOutboundFiles(replyCtx, appID, senderID, groupOpenID, scope, out.FileNames, msgType)
	}

	if rec := imhistory.GlobalRecorder(); rec != nil && sessionID != "" {
		rec.RecordTurnAsync(cfg.AgentID, sessionID, imUserID, userInput, respText)
	}
}

func (c *Client) getEntry(appID string) *BotEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bots[appID]
}

func isSenderAllowed(senderID string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, id := range allowed {
		if id == senderID {
			return true
		}
	}
	return false
}

// ---- Stop / cleanup ----

func (c *Client) StopBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.bots[appID]
	if !ok || entry == nil {
		return
	}
	closeBotWSEntry(entry)
	c.syncRunningFlag()
	logger.Info("qqbot: ws client stopped for bot", "app_id", appID)
}

func (c *Client) Stop() {
	c.mu.Lock()
	for key := range c.bots {
		closeBotWSEntry(c.bots[key])
		logger.Info("qqbot: ws client stopped", "key", key)
	}
	c.syncRunningFlag()
	c.mu.Unlock()
	logger.Info("qqbot: all ws clients stopped")
}

func closeBotWSEntry(entry *BotEntry) {
	if entry == nil {
		return
	}
	if entry.wsConn != nil {
		entry.wsConn.Close()
		entry.wsConn = nil
	}
	if entry.wsCancel != nil {
		entry.wsCancel()
		entry.wsCancel = nil
	}
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
