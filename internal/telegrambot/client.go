package telegrambot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/imhistory"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/skills"
	"github.com/larksuite/oapi-sdk-go/v3/channel/safety"
)

type BotConfig struct {
	Token          string
	ChatID         string
	AgentID        int64
	InvokeTimeout  int
	WebhookURL     string
	WebhookEnabled bool
	WsEnabled      bool
	AllowedUsers   []string
}

type BotEntry struct {
	Config     *BotConfig
	Runtime    *agent.Runtime
	pollCancel context.CancelFunc
	running    bool
}

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

func (c *Client) RegisterBot(cfg *BotConfig, runtime *agent.Runtime) error {
	if cfg == nil || cfg.Token == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: token and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.bots[cfg.Token]; exists {
		if existing.Config.AgentID != cfg.AgentID {
			logger.Warn("telegrambot: token already bound to another agent",
				"token_prefix", cfg.Token[:8], "new_agent_id", cfg.AgentID, "existing_agent_id", existing.Config.AgentID)
			return fmt.Errorf("token already bound to agent %d", existing.Config.AgentID)
		}
		existing.Config = cfg
		existing.Runtime = runtime
		logger.Info("telegrambot: bot re-registered", "agent_id", cfg.AgentID)
		return nil
	}

	c.bots[cfg.Token] = &BotEntry{
		Config:  cfg,
		Runtime: runtime,
	}

	logger.Info("telegrambot: bot registered", "agent_id", cfg.AgentID, "has_chat_id", cfg.ChatID != "", "webhook", cfg.WebhookEnabled)
	return nil
}

func (c *Client) UnregisterBot(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[token]
	if ok {
		c.stopEntryLocked(entry)
		delete(c.bots, token)
		logger.Info("telegrambot: bot unregistered", "token_prefix", token[:8])
	}
}

func (c *Client) UpdateBotConfig(cfg *BotConfig) error {
	if cfg == nil || cfg.Token == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: token and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.bots[cfg.Token]
	if !exists {
		return fmt.Errorf("bot not found for token")
	}

	if entry.Config.AgentID != cfg.AgentID {
		return fmt.Errorf("agent_id mismatch: cannot change agent binding")
	}

	entry.Config = cfg
	logger.Info("telegrambot: bot config updated", "agent_id", cfg.AgentID)
	return nil
}

func (c *Client) GetBotConfig(token string) *BotConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[token]; ok {
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

func (c *Client) Start(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.bots) == 0 {
		logger.Info("telegrambot: no bots registered, skip starting")
		return
	}

	started := 0
	for token, entry := range c.bots {
		if entry == nil || entry.Config == nil {
			continue
		}
		if !entry.Config.WsEnabled {
			logger.Info("telegrambot: skip global start (ws_enabled false)", "agent_id", entry.Config.AgentID)
			continue
		}
		if err := c.startEntryLocked(ctx, token, entry); err != nil {
			logger.Warn("telegrambot: start bot failed", "agent_id", entry.Config.AgentID, "err", err)
			continue
		}
		started++
	}

	if started > 0 {
		c.running = true
	}
	logger.Info("telegrambot: global start complete", "started", started, "bot_count", len(c.bots))
}

func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range c.bots {
		c.stopEntryLocked(entry)
	}
	c.running = false
	logger.Info("telegrambot: all bots stopped")
}

func (c *Client) StartBot(ctx context.Context, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[token]
	if !ok || entry == nil {
		return fmt.Errorf("bot not found for token")
	}
	if entry.running {
		return nil
	}
	if err := c.startEntryLocked(ctx, token, entry); err != nil {
		return err
	}
	c.running = true
	logger.Info("telegrambot: bot started", "agent_id", entry.Config.AgentID)
	return nil
}

func (c *Client) StopBot(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[token]
	if !ok || entry == nil {
		return
	}
	c.stopEntryLocked(entry)
	logger.Info("telegrambot: bot stopped", "agent_id", entry.Config.AgentID)
}

func (c *Client) IsBotRunning(token string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.bots[token]
	return ok && entry != nil && entry.running
}

func (c *Client) startEntryLocked(ctx context.Context, token string, entry *BotEntry) error {
	cfg := entry.Config
	if cfg.WebhookEnabled && cfg.WebhookURL != "" {
		if err := c.setWebhook(ctx, token, cfg.WebhookURL); err != nil {
			return err
		}
		entry.running = true
		return nil
	}

	if entry.pollCancel != nil {
		entry.running = true
		return nil
	}

	pollCtx, cancel := context.WithCancel(context.Background())
	entry.pollCancel = cancel
	entry.running = true
	go c.pollLoop(pollCtx, token)
	return nil
}

func (c *Client) stopEntryLocked(entry *BotEntry) {
	if entry == nil {
		return
	}
	if entry.pollCancel != nil {
		entry.pollCancel()
		entry.pollCancel = nil
	}
	entry.running = false
}

func (c *Client) pollLoop(ctx context.Context, token string) {
	offset := int64(0)
	const maxBackoff = 30 * time.Second
	backoff := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := c.fetchUpdates(ctx, token, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("telegrambot: getUpdates failed", "token_prefix", tokenPrefix(token), "err", err, "backoff", backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = 1 * time.Second

		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			body, err := json.Marshal(u)
			if err != nil {
				continue
			}
			if err := c.HandleUpdate(context.Background(), token, body); err != nil {
				logger.Warn("telegrambot: handle update failed", "err", err)
			}
		}
	}
}

func (c *Client) fetchUpdates(ctx context.Context, token string, offset int64, timeoutSec int) ([]Update, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=%d", token, offset, timeoutSec)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getUpdates status %d: %s", resp.StatusCode, truncateBytes(body, 300))
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("getUpdates not ok")
	}
	return result.Result, nil
}

func (c *Client) setWebhook(ctx context.Context, token, webhookURL string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", token)
	payload, err := json.Marshal(map[string]string{"url": webhookURL})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook setup failed: %s", body)
	}

	logger.Info("telegrambot: webhook set", "url", webhookURL)
	return nil
}

func (c *Client) HandleUpdate(ctx context.Context, botToken string, payload []byte) error {
	if strings.TrimSpace(botToken) == "" {
		return fmt.Errorf("bot token required")
	}

	var update Update
	if err := json.Unmarshal(payload, &update); err != nil {
		logger.Warn("telegrambot: failed to parse update", "err", err)
		return err
	}

	if update.Message == nil {
		return nil
	}

	msg := update.Message
	if msg.From != nil && msg.From.IsBot {
		return nil
	}

	dedupKey := fmt.Sprintf("%s:%d", tokenPrefix(botToken), msg.MessageID)
	if c.msgDedup != nil && c.msgDedup.IsDuplicate(dedupKey) {
		logger.Info("telegrambot: duplicate message, skipping", "message_id", msg.MessageID)
		return nil
	}

	userInput := strings.TrimSpace(msg.Text)
	if userInput == "" {
		logger.Info("telegrambot: empty message, ignoring")
		return nil
	}

	cfg := c.GetBotConfig(botToken)
	if cfg == nil {
		logger.Warn("telegrambot: no bot config for token", "token_prefix", tokenPrefix(botToken))
		return nil
	}

	senderID := telegramSenderID(msg.From)
	if !isSenderAllowed(senderID, cfg.AllowedUsers) {
		logger.Info("telegrambot: sender not in allowed_users", "sender_id", senderID, "agent_id", cfg.AgentID)
		return nil
	}

	runtime := c.GetBotRuntime(botToken)
	if runtime == nil {
		logger.Error("telegrambot: no bot runtime for token", "token_prefix", tokenPrefix(botToken))
		return nil
	}

	chatID := getChatID(msg.Chat)
	if chatID == "" && cfg.ChatID != "" {
		chatID = cfg.ChatID
	}

	var sessionID string
	var history []*einoschema.Message
	imUserID := imhistory.FormatIMUserID("telegram", senderID)
	if rec := imhistory.GlobalRecorder(); rec != nil && senderID != "" {
		sessionID, history = rec.PrepareConversation(ctx, "telegram", cfg.AgentID, senderID, 20)
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

	respText, err := c.invokeAgent(ctx, runtime, cfg.AgentID, userInput, senderID, sessionID, history)
	if err != nil {
		respText = fmt.Sprintf("处理消息失败: %v", err)
		logger.Error("telegrambot: invoke agent failed", "err", err)
	}

	scope := imoutbound.Scope{AgentID: cfg.AgentID, SessionID: sessionID}
	out := imoutbound.DeliverIMReply(imoutbound.DeliverInput{
		Channel: "telegram", Ctx: ctx, AgentCtx: ctx, Scope: scope, Store: imoutbound.GlobalStore(),
		UserRequest: userInput, AgentText: respText,
	})

	if chatID != "" && strings.TrimSpace(out.Text) != "" {
		if err := c.SendMessage(ctx, botToken, chatID, out.Text); err != nil {
			logger.Warn("telegrambot: send reply failed", "err", err)
		}
	}
	if chatID != "" && len(out.FileNames) > 0 {
		c.sendOutboundFiles(ctx, botToken, chatID, scope, out.FileNames)
	}

	if rec := imhistory.GlobalRecorder(); rec != nil && sessionID != "" {
		rec.RecordTurnAsync(cfg.AgentID, sessionID, imUserID, userInput, respText)
	}

	return nil
}

func (c *Client) invokeAgent(ctx context.Context, runtime *agent.Runtime, agentID int64, userInput, auditUserID, sessionID string, history []*einoschema.Message) (string, error) {
	resp, err := runtime.ChatWithMemoryContext(ctx, agentID, userInput, "", "", imoutbound.IMMemoryContextHint(), sessionID, auditUserID, history)
	return resp, err
}

func (c *Client) sendOutboundFiles(ctx context.Context, token, chatID string, scope imoutbound.Scope, names []string) {
	store := imoutbound.GlobalStore()
	const maxFilesPerReply = 5
	if len(names) > maxFilesPerReply {
		names = names[:maxFilesPerReply]
	}
	for _, name := range names {
		abs, err := skills.ResolveIMAttachmentPath(store, scope, name)
		if err != nil {
			logger.Warn("telegrambot: outbound file resolve failed", "file", name, "err", err)
			continue
		}
		if err := c.sendTelegramFile(ctx, token, chatID, abs); err != nil {
			logger.Warn("telegrambot: send file failed", "file", name, "err", err)
		} else {
			logger.Info("telegrambot: file sent", "file", name)
		}
	}
}

func (c *Client) sendTelegramFile(ctx context.Context, token, chatID, absPath string) error {
	ext := filepath.Ext(absPath)
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".bmp"

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, map[bool]string{true: "sendPhoto", false: "sendDocument"}[isImage])

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("chat_id", chatID); err != nil {
		return err
	}
	part, err := writer.CreateFormFile(map[bool]string{true: "photo", false: "document"}[isImage], filepath.Base(absPath))
	if err != nil {
		return err
	}
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram send file failed: %s", b)
	}
	return nil
}

func (c *Client) SendMessage(ctx context.Context, token, chatID, text string) error {
	if chatID == "" {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := map[string]string{
		"chat_id": chatID,
		"text":    text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send message failed: %s", b)
	}

	return nil
}

func (c *Client) GetBotRuntime(token string) *agent.Runtime {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[token]; ok {
		return entry.Runtime
	}
	return nil
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      *Chat  `json:"chat"`
	From      *User  `json:"from,omitempty"`
	Text      string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID       int64  `json:"id"`
	IsBot    bool   `json:"is_bot"`
	Username string `json:"username,omitempty"`
}

func getChatID(chat *Chat) string {
	if chat == nil {
		return ""
	}
	return strconv.FormatInt(chat.ID, 10)
}

func telegramSenderID(from *User) string {
	if from == nil {
		return ""
	}
	return strconv.FormatInt(from.ID, 10)
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

func tokenPrefix(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8]
}

func truncateBytes(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max])
}

// WebhookPath returns the inbound webhook path for an agent (HTTPS required in production).
func WebhookPath(agentID int64) string {
	return fmt.Sprintf("/api/v1/telegrambots/webhook/%d", agentID)
}

// WebhookURL builds a full webhook URL from a public base URL.
func WebhookURL(base string, agentID int64) string {
	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	return base + WebhookPath(agentID)
}

// ValidateWebhookURL ensures the configured webhook points at this server path for the agent.
func ValidateWebhookURL(webhookURL string, agentID int64) bool {
	u, err := url.Parse(strings.TrimSpace(webhookURL))
	if err != nil || u.Scheme != "https" {
		return false
	}
	return strings.HasSuffix(strings.TrimSuffix(u.Path, "/"), fmt.Sprintf("/webhook/%d", agentID))
}
