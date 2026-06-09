package workflow

import (
	"testing"

	"github.com/fisk086/aiops/internal/model"
)

func TestPrepareNodeConfigMergesInputValuesIntoArguments(t *testing.T) {
	node := &model.WorkflowNode{
		ID:   "mcp1",
		Type: string(TaskTypeMCP),
		Config: map[string]any{
			"arguments": "{}",
			"input_values": map[string]any{
				"query": "{{sys.query}}",
			},
		},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
		},
	}
	execCtx := NewExecutionContext()
	execCtx.UserMessage = "hello"
	execCtx.VarContext.SetSystemVariable("sys.query", "hello")

	cfg := prepareNodeConfig(node, execCtx)
	args, err := parseArgumentsObject(cfg)
	if err != nil {
		t.Fatalf("parse arguments: %v", err)
	}
	if args["query"] != "hello" {
		t.Fatalf("expected resolved query, got %v", args["query"])
	}
}

func TestValidateInputSchemaAcceptsInputValues(t *testing.T) {
	engine := &GraphEngine{}
	node := &model.WorkflowNode{
		ID:   "n1",
		Type: string(TaskTypeAgent),
		Config: map[string]any{
			"input_values": map[string]any{
				"prompt": "hello",
			},
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"prompt"},
			"properties": map[string]any{
				"prompt": map[string]any{"type": "string", "required": true},
			},
		},
	}
	ctx := NewExecutionContext()
	if err := engine.validateInputSchema(node, ctx); err != nil {
		t.Fatalf("validateInputSchema: %v", err)
	}
}
