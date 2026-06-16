package service

import (
	"context"

	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/storage"
)

type AgentService struct {
	store storage.Storage
}

func NewAgentService(store storage.Storage) *AgentService {
	return &AgentService{store: store}
}

func (s *AgentService) ListAgents() ([]*schema.Agent, error) {
	return s.store.ListAgents()
}

func (s *AgentService) ListAgentsForSchedule() ([]*schema.Agent, error) {
	agents, err := s.store.ListAgents()
	if err != nil {
		return nil, err
	}
	result := make([]*schema.Agent, 0, len(agents))
	for _, a := range agents {
		full, err := s.store.GetAgent(a.ID)
		if err != nil || full == nil {
			continue
		}
		if full.RuntimeProfile != nil && full.RuntimeProfile.ExecutionMode == schema.ExecutionModeClient {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

func (s *AgentService) GetAgent(id int64) (*schema.AgentWithRuntime, error) {
	return s.store.GetAgent(id)
}

func (s *AgentService) CreateAgent(req *schema.CreateAgentRequest, createdBy int64) (*schema.Agent, error) {
	agent, err := s.store.CreateAgent(req, createdBy)
	if err != nil {
		return nil, err
	}

	if len(req.ChatUserIDs) > 0 {
		if cerr := s.store.SetAgentChatUsers(context.Background(), agent.ID, req.ChatUserIDs); cerr != nil {
			logger.Warn("failed to set chat users on create", "agent_id", agent.ID, "err", cerr)
		}
	}

	if req.RuntimeProfile != nil {
		full, gerr := s.store.GetAgent(agent.ID)
		if gerr != nil {
			logger.Warn("runtime_profile persistence check skipped: could not reload agent after create", "agent_id", agent.ID, "error", gerr)
		} else if full != nil && full.RuntimeProfile == nil {
			logger.Warn("runtime_profile was sent but agent has no persisted runtime row after create", "agent_id", agent.ID)
		} else if full != nil && full.RuntimeProfile != nil {
			logRuntimeProfilePersistenceCheck("create", agent.ID, req.RuntimeProfile, full.RuntimeProfile)
		}
	}
	return agent, nil
}

func (s *AgentService) UpdateAgent(id int64, req *schema.UpdateAgentRequest) (*schema.Agent, error) {
	agent, err := s.store.UpdateAgent(id, req)
	if err != nil {
		return nil, err
	}

	if req.ChatUserIDs != nil {
		if cerr := s.store.SetAgentChatUsers(context.Background(), id, req.ChatUserIDs); cerr != nil {
			logger.Warn("failed to set chat users on update", "agent_id", id, "err", cerr)
		}
	}

	if req.RuntimeProfile != nil {
		full, gerr := s.store.GetAgent(id)
		if gerr != nil {
			logger.Warn("runtime_profile persistence check skipped: could not reload agent after update", "agent_id", id, "error", gerr)
		} else if full != nil && full.RuntimeProfile == nil {
			logger.Warn("runtime_profile was sent but agent has no persisted runtime row after update", "agent_id", id)
		} else if full != nil && full.RuntimeProfile != nil {
			logRuntimeProfilePersistenceCheck("update", id, req.RuntimeProfile, full.RuntimeProfile)
		}
	}
	return agent, nil
}

func (s *AgentService) CanChat(ctx context.Context, agentID, userID int64) (bool, error) {
	ownerID, err := s.store.GetAgentOwner(agentID)
	if err != nil {
		return false, err
	}
	if ownerID == userID {
		return true, nil
	}
	users, err := s.store.GetAgentChatUsers(ctx, agentID)
	if err != nil {
		return false, err
	}
	for _, uid := range users {
		if uid == userID {
			return true, nil
		}
	}
	return false, nil
}

func (s *AgentService) GetAgentOwner(id int64) (int64, error) {
	return s.store.GetAgentOwner(id)
}

func (s *AgentService) DeleteAgent(id int64) error {
	return s.store.DeleteAgent(id)
}

func (s *AgentService) GetCapabilityTree(id int64) (*schema.CapabilityTree, error) {
	return s.store.GetCapabilityTree(id)
}

func (s *AgentService) UpdateCapabilityTree(id int64, nodes []schema.CapabilityTreeNode) (*schema.CapabilityTree, error) {
	return s.store.UpdateCapabilityTree(id, nodes)
}
