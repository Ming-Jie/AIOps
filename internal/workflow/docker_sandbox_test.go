package workflow

import (
	"strings"
	"testing"
)

func TestGenerateRunnerCodePythonInjectsMappingGlobals(t *testing.T) {
	inputJSON := mapToJSON(map[string]any{
		"mapping": map[string]any{
			"body": `{"ip_addr":"1.2.3.4"}`,
		},
	})
	code := generateRunnerCode("python", "print(body)", inputJSON)
	if !strings.Contains(code, `_mapping = input_data.get("mapping")`) {
		t.Fatalf("expected mapping injection wrapper, got:\n%s", code)
	}
	if !strings.Contains(code, "print(body)") {
		t.Fatalf("expected user code preserved, got:\n%s", code)
	}
}

func TestGenerateRunnerCodeJavaScriptInjectsMappingGlobals(t *testing.T) {
	inputJSON := mapToJSON(map[string]any{
		"mapping": map[string]any{
			"body": "hello",
		},
	})
	code := generateRunnerCode("javascript", "console.log(body)", inputJSON)
	if !strings.Contains(code, "globalThis[k] = v") {
		t.Fatalf("expected mapping injection wrapper, got:\n%s", code)
	}
}
