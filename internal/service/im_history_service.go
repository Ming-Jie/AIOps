package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/aiops/internal/imhistory"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/storage"
)

type IMHistoryService struct {
	store storage.Storage
}

func NewIMHistoryService(store storage.Storage) *IMHistoryService {
	return &IMHistoryService{store: store}
}

func (s *IMHistoryService) ListIMSessions(ctx context.Context, agentID int64, channel string, limit, offset int) ([]imhistory.IMChatSession, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store not available")
	}
	if agentID < 1 {
		return nil, fmt.Errorf("agent_id must be >= 1")
	}
	prefix := imhistory.UserIDPrefixForChannel(channel)
	list, err := s.store.ListChatSessionsByUserPrefix(ctx, agentID, prefix, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]imhistory.IMChatSession, 0, len(list))
	for _, sess := range list {
		if !imhistory.IsIMUserID(sess.UserID) {
			continue
		}
		out = append(out, imhistory.EnrichSession(sess))
	}
	return out, nil
}

func (s *IMHistoryService) GetIMSession(ctx context.Context, agentID int64, sessionID string) (*imhistory.IMChatSession, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store not available")
	}
	sess, err := s.store.GetChatSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.AgentID != agentID {
		return nil, storage.ErrSessionForbidden
	}
	if !imhistory.IsIMUserID(sess.UserID) {
		return nil, storage.ErrSessionForbidden
	}
	enriched := imhistory.EnrichSession(*sess)
	return &enriched, nil
}

func (s *IMHistoryService) ListIMMessages(ctx context.Context, agentID int64, sessionID string, limit, offset int) ([]schema.ChatHistoryMessage, error) {
	if _, err := s.GetIMSession(ctx, agentID, sessionID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session_id required")
	}
	if offset > 0 || limit > 500 {
		if limit <= 0 || limit > 2000 {
			limit = 500
		}
		return s.store.ListSessionMessagesPage(ctx, sessionID, offset, limit)
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.store.ListRecentSessionMessages(ctx, sessionID, limit)
}
