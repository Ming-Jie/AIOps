package agent

import (
	"context"
	"errors"
	"sync"

	"github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/schema"
)

type agentModelKey struct{}

func contextWithAgentID(ctx context.Context, agentID int64) context.Context {
	return context.WithValue(ctx, agentModelKey{}, agentID)
}

func agentIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(agentModelKey{}).(int64)
	return id
}

type resolvableModel struct {
	defaultModel model.ToolCallingChatModel
	provider     ModelProvider
	agents       func(int64) (*schema.AgentWithRuntime, bool)
	cache        sync.Map
	modelNames   sync.Map
}

var errNoModelConfig = errors.New("agent has no model config bound")

func (m *resolvableModel) resolve(ctx context.Context) (model.ToolCallingChatModel, string, error) {
	aid := agentIDFromContext(ctx)
	if aid == 0 || m.provider == nil {
		return nil, "", errNoModelConfig
	}
	ag, ok := m.agents(aid)
	if !ok || ag.RuntimeProfile == nil || ag.RuntimeProfile.ModelConfigID == 0 {
		return nil, "", errNoModelConfig
	}
	cid := ag.RuntimeProfile.ModelConfigID
	if cached, ok := m.cache.Load(cid); ok {
		if mnVal, ok := m.modelNames.Load(cid); ok {
			return cached.(model.ToolCallingChatModel), mnVal.(string), nil
		}
		return cached.(model.ToolCallingChatModel), "", nil
	}
	chatModel, err := m.provider.GetChatModel(ctx, cid)
	if err != nil {
		return nil, "", err
	}
	wrapped := WrapToolCallingModelWithUsageTracking(chatModel)
	m.cache.Store(cid, wrapped)
	mn, err := m.provider.ResolveModelName(ctx, cid)
	if err != nil || mn == "" {
		mn = "unknown"
	}
	m.modelNames.Store(cid, mn)
	return wrapped, mn, nil
}

func (m *resolvableModel) Generate(ctx context.Context, msgs []*einoschema.Message, opts ...model.Option) (*einoschema.Message, error) {
	chatModel, modelName, err := m.resolve(ctx)
	if err != nil {
		if errors.Is(err, errNoModelConfig) && m.defaultModel != nil {
			return m.defaultModel.Generate(ctx, msgs, opts...)
		}
		return nil, err
	}
	setUsageModelName(ctx, modelName)
	return chatModel.Generate(ctx, msgs, opts...)
}

func (m *resolvableModel) Stream(ctx context.Context, msgs []*einoschema.Message, opts ...model.Option) (*einoschema.StreamReader[*einoschema.Message], error) {
	chatModel, modelName, err := m.resolve(ctx)
	if err != nil {
		if errors.Is(err, errNoModelConfig) && m.defaultModel != nil {
			return m.defaultModel.Stream(ctx, msgs, opts...)
		}
		return nil, err
	}
	setUsageModelName(ctx, modelName)
	return chatModel.Stream(ctx, msgs, opts...)
}

func (m *resolvableModel) WithTools(tools []*einoschema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
