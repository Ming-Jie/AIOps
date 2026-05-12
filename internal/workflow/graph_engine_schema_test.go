package workflow

import (
	"strings"
	"testing"

	"github.com/fisk086/sya/internal/model"
)

func TestValidateInputSchemaNoMappingDoesNotPanicWhenUserMessageExists(t *testing.T) {
	engine := &GraphEngine{}
	node := &model.WorkflowNode{
		ID:     "agent_1",
		Config: map[string]any{},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"city"},
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
		},
	}
	ctx := NewExecutionContext()
	ctx.UserMessage = "Shanghai"

	if err := engine.validateInputSchema(node, ctx); err != nil {
		t.Fatalf("validateInputSchema returned error: %v", err)
	}
}

func TestValidateInputSchemaRequiresStandardJSONSchemaRequiredFields(t *testing.T) {
	engine := &GraphEngine{}
	node := &model.WorkflowNode{
		ID:     "agent_1",
		Config: map[string]any{},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"city"},
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
		},
	}
	ctx := NewExecutionContext()

	err := engine.validateInputSchema(node, ctx)
	if err == nil {
		t.Fatal("expected required field error")
	}
	if !strings.Contains(err.Error(), "required field 'city'") {
		t.Fatalf("unexpected error: %v", err)
	}
}
