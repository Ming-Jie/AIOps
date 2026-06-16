package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/fisk086/aiops/internal/embedding"
	appmodel "github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/storage"
)

type ModelService struct {
	store      storage.ModelConfigStore
	cache      sync.Map
	modelNames sync.Map
}

func NewModelService(store storage.ModelConfigStore) *ModelService {
	return &ModelService{store: store}
}

func (s *ModelService) Create(ctx context.Context, cfg *appmodel.ModelConfig) (*appmodel.ModelConfig, error) {
	return s.store.CreateModelConfig(ctx, cfg)
}

func (s *ModelService) Get(ctx context.Context, id int64) (*appmodel.ModelConfig, error) {
	return s.store.GetModelConfig(ctx, id)
}

func (s *ModelService) List(ctx context.Context) ([]*appmodel.ModelConfig, error) {
	return s.store.ListModelConfigs(ctx)
}

func (s *ModelService) Update(ctx context.Context, id int64, cfg *appmodel.ModelConfig) (*appmodel.ModelConfig, error) {
	s.cache.Delete(id)
	s.modelNames.Delete(id)
	return s.store.UpdateModelConfig(ctx, id, cfg)
}

func (s *ModelService) Delete(ctx context.Context, id int64) error {
	s.cache.Delete(id)
	s.modelNames.Delete(id)
	return s.store.DeleteModelConfig(ctx, id)
}

func (s *ModelService) GetChatModel(ctx context.Context, configID int64) (model.ToolCallingChatModel, error) {
	if cached, ok := s.cache.Load(configID); ok {
		if m, ok := cached.(model.ToolCallingChatModel); ok {
			return m, nil
		}
	}
	cfg, err := s.store.GetModelConfig(ctx, configID)
	if err != nil {
		return nil, fmt.Errorf("model config %d: %w", configID, err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("model config %d not found", configID)
	}
	if cfg.Purpose != "" && cfg.Purpose != "chat" {
		return nil, fmt.Errorf("model config %d is not a chat model (purpose=%s)", configID, cfg.Purpose)
	}
	m, err := s.buildChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build model %d: %w", configID, err)
	}
	s.cache.Store(configID, m)
	return m, nil
}

func (s *ModelService) ResolveModelName(ctx context.Context, configID int64) (string, error) {
	if cached, ok := s.modelNames.Load(configID); ok {
		if name, ok := cached.(string); ok && name != "" {
			return name, nil
		}
	}
	cfg, err := s.store.GetModelConfig(ctx, configID)
	if err != nil {
		return "", fmt.Errorf("model config %d: %w", configID, err)
	}
	if cfg == nil {
		return "", fmt.Errorf("model config %d not found", configID)
	}
	s.modelNames.Store(configID, cfg.ModelName)
	return cfg.ModelName, nil
}

func (s *ModelService) buildChatModel(ctx context.Context, cfg *appmodel.ModelConfig) (model.ToolCallingChatModel, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "openai":
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  cfg.APIKey,
			Model:   cfg.ModelName,
			BaseURL: cfg.BaseURL,
		})
	case "ark":
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey:  cfg.APIKey,
			Model:   cfg.ModelName,
			BaseURL: cfg.BaseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported chat model provider: %s", cfg.Provider)
	}
}

// GetActiveEmbeddingConfig returns the first active embedding model config.
func (s *ModelService) GetActiveEmbeddingConfig(ctx context.Context) (*appmodel.ModelConfig, error) {
	list, err := s.store.ListModelConfigsByPurpose(ctx, "embedding")
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		if c.IsActive {
			return c, nil
		}
	}
	return nil, nil
}

// BuildEmbeddingService creates an embedding.Service from a model config.
func (s *ModelService) BuildEmbeddingService(cfg *appmodel.ModelConfig, dimension int) *embedding.Service {
	if cfg == nil {
		return nil
	}
	return embedding.NewService(cfg.APIKey, cfg.ModelName, cfg.BaseURL, dimension)
}


