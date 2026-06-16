package agent

import (
	"context"

	"github.com/cloudwego/eino/components/model"
)

type ModelProvider interface {
	GetChatModel(ctx context.Context, configID int64) (model.ToolCallingChatModel, error)
	ResolveModelName(ctx context.Context, configID int64) (string, error)
}
