package workflow

import "testing"

func TestDeclaredOutputsStartFromInputFields(t *testing.T) {
	config := map[string]any{
		"input_fields": []any{
			map[string]any{"name": "query", "type": "text", "label": "Query"},
			map[string]any{"name": "top_k", "type": "number", "label": "Top K"},
		},
	}
	got := DeclaredOutputs("start", config)
	for _, want := range []string{"message", "type", "query", "top_k"} {
		if !containsString(got, want) {
			t.Fatalf("expected %q in %v", want, got)
		}
	}
}

func TestDeclaredOutputsLlmOutputVar(t *testing.T) {
	got := DeclaredOutputs("llm", map[string]any{"output_var": "answer"})
	if !containsString(got, "answer") {
		t.Fatalf("expected answer in %v", got)
	}
}

func TestDeclaredOutputsCodeOutputs(t *testing.T) {
	got := DeclaredOutputs("code", map[string]any{
		"outputs": map[string]any{"result": "string", "count": "number"},
	})
	for _, want := range []string{"result", "count", "output"} {
		if !containsString(got, want) {
			t.Fatalf("expected %q in %v", want, got)
		}
	}
}
