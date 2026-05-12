package workflow

import (
	"testing"
	"time"
)

func TestMCPArgumentsFromConfigParsesJSONBeforeTemplateReplacement(t *testing.T) {
	ctx := NewVariableContext()
	query := "status: \"critical\"\nregion=cn"
	ctx.SetSystemVariable("sys.query", query)

	args, err := mcpArgumentsFromConfig(&TaskInput{
		Config: map[string]any{
			"arguments": `{"query":"{{sys.query}}","filters":["{{sys.query}}"],"nested":{"q":"{{sys.query}}"}}`,
		},
		VarContext: ctx,
	})
	if err != nil {
		t.Fatalf("mcpArgumentsFromConfig returned error: %v", err)
	}

	if got := args["query"]; got != query {
		t.Fatalf("query = %#v, want %#v", got, query)
	}

	filters, ok := args["filters"].([]any)
	if !ok || len(filters) != 1 || filters[0] != query {
		t.Fatalf("filters = %#v, want query in array", args["filters"])
	}

	nested, ok := args["nested"].(map[string]any)
	if !ok || nested["q"] != query {
		t.Fatalf("nested = %#v, want query in nested map", args["nested"])
	}
}

func TestMCPArgumentsFromConfigRejectsNonObjectJSON(t *testing.T) {
	_, err := mcpArgumentsFromConfig(&TaskInput{
		Config: map[string]any{
			"arguments": `["not", "object"]`,
		},
		VarContext: NewVariableContext(),
	})
	if err == nil {
		t.Fatal("expected error for non-object JSON arguments")
	}
}

func TestConvertToModelResultsPreservesExecutionMetadata(t *testing.T) {
	start := time.Date(2026, 5, 9, 10, 30, 0, 0, time.UTC)
	end := start.Add(250 * time.Millisecond)

	got := convertToModelResults([]NodeResult{{
		NodeID:     "mcp_1",
		Label:      "MCP Lookup",
		NodeType:   "mcp",
		Input:      "input text",
		Output:     map[string]any{"type": "mcp", "result": "ok"},
		Error:      "",
		StartTime:  start,
		EndTime:    end,
		DurationMs: 250,
		RetryCount: 1,
	}})

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	row := got[0]
	if row.NodeType != "mcp" || row.Input != "input text" || row.DurationMs != 250 || row.RetryCount != 1 {
		t.Fatalf("metadata not preserved: %#v", row)
	}
	if row.StartTime == "" || row.EndTime == "" {
		t.Fatalf("time fields not preserved: %#v", row)
	}
	if row.Output["result"] != "ok" || row.Output["data"] != nil {
		t.Fatalf("output = %#v, want direct output map", row.Output)
	}
}
