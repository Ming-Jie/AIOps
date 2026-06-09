package imhistory

import (
	"context"
	"strings"
	"sync"
	"time"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/chathistory"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/memory"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/storage"
)

// Recorder persists IM bot turns into chat_sessions + agent_memory.
type Recorder struct {
	store     storage.Storage
	mem       memory.Provider
	vectorDim int
	wg        sync.WaitGroup
}

func NewRecorder(store storage.Storage, mem memory.Provider, vectorDim int) *Recorder {
	if vectorDim <= 0 {
		vectorDim = 1536
	}
	return &Recorder{store: store, mem: mem, vectorDim: vectorDim}
}

func (r *Recorder) Wait() {
	r.wg.Wait()
}

func (r *Recorder) isMemoryEnabled(agentID int64) bool {
	if r.store == nil || agentID <= 0 {
		return false
	}
	agent, err := r.store.GetAgent(agentID)
	if err != nil || agent == nil || agent.RuntimeProfile == nil {
		return false
	}
	return agent.RuntimeProfile.MemoryEnabled
}

func (r *Recorder) storeTurn(ctx context.Context, agentID int64, userID, sessionID, role, content string, extra map[string]any, useEmbedding bool) {
	content = strings.TrimSpace(content)
	if content == "" || strings.TrimSpace(sessionID) == "" || r.store == nil {
		return
	}
	zero := make([]float32, r.vectorDim)
	if useEmbedding && r.mem != nil {
		if err := r.mem.Record(ctx, memory.Turn{
			AgentID: agentID, UserID: userID, SessionID: sessionID, Role: role, Content: content, Extra: extra,
		}); err != nil {
			logger.Warn("imhistory: memory record failed, fallback store", "agent_id", agentID, "session_id", sessionID, "role", role, "err", err)
			if err2 := r.store.StoreMemory(ctx, agentID, userID, sessionID, role, content, zero, extra); err2 != nil {
				logger.Error("imhistory: store turn fallback failed", "agent_id", agentID, "session_id", sessionID, "err", err2)
			}
		}
		return
	}
	if err := r.store.StoreMemory(ctx, agentID, userID, sessionID, role, content, zero, extra); err != nil {
		logger.Error("imhistory: store turn failed", "agent_id", agentID, "session_id", sessionID, "role", role, "err", err)
	}
}

func (r *Recorder) ensureSession(ctx context.Context, agentID int64, imUserID string) (string, error) {
	if r.store == nil || agentID <= 0 || strings.TrimSpace(imUserID) == "" {
		return "", nil
	}
	list, err := r.store.ListChatSessions(ctx, agentID, imUserID, 1, 0)
	if err != nil {
		return "", err
	}
	if len(list) > 0 {
		return list[0].SessionID, nil
	}
	sess, err := r.store.CreateChatSession(ctx, agentID, imUserID, 0)
	if err != nil {
		return "", err
	}
	return sess.SessionID, nil
}

// PrepareConversation returns a stable session_id and recent history for multi-turn IM chat.
func (r *Recorder) PrepareConversation(ctx context.Context, channel string, agentID int64, externalUserID string, historyLimit int) (sessionID string, history []*einoschema.Message) {
	externalUserID = strings.TrimSpace(externalUserID)
	if externalUserID == "" || r.store == nil {
		return "", nil
	}
	imUserID := FormatIMUserID(channel, externalUserID)
	sid, err := r.ensureSession(ctx, agentID, imUserID)
	if err != nil {
		logger.Warn("imhistory: ensure session failed", "agent_id", agentID, "channel", channel, "err", err)
		return "", nil
	}
	if sid == "" {
		return "", nil
	}
	if historyLimit <= 0 {
		historyLimit = 20
	}
	msgs, err := r.store.ListRecentSessionMessages(ctx, sid, historyLimit)
	if err != nil || len(msgs) == 0 {
		return sid, nil
	}
	return sid, chathistory.ToEinoMessages(msgs)
}

// RecordTurnAsync stores user + assistant messages without blocking the IM handler.
func (r *Recorder) RecordTurnAsync(agentID int64, sessionID, imUserID, userMsg, assistantMsg string) {
	if r == nil || r.store == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	userMsg = strings.TrimSpace(userMsg)
	assistantMsg = strings.TrimSpace(assistantMsg)
	if userMsg == "" && assistantMsg == "" {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		extra := map[string]any{"source": "im_bot"}
		useEmbedding := r.isMemoryEnabled(agentID) && r.mem != nil
		r.storeTurn(ctx, agentID, imUserID, sessionID, "user", userMsg, extra, useEmbedding)
		r.storeTurn(ctx, agentID, imUserID, sessionID, "assistant", assistantMsg, extra, useEmbedding)
	}()
}

// IMChatSession enriches ChatSession for the admin UI.
type IMChatSession struct {
	schema.ChatSession
	IMChannel string `json:"im_channel"`
	IMUserID  string `json:"im_user_id"`
}

func EnrichSession(sess schema.ChatSession) IMChatSession {
	out := IMChatSession{ChatSession: sess}
	if ch, ext, ok := ParseIMUserID(sess.UserID); ok {
		out.IMChannel = ch
		out.IMUserID = ext
	}
	return out
}
