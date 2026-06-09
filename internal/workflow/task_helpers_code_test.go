package workflow

import (
	"context"
	"strings"
	"testing"
)

func TestExecutePythonInjectsInputMapping(t *testing.T) {
	input := map[string]any{
		"mapping": map[string]any{
			"body": `{"ip_addr":"54.89.198.138"}`,
		},
	}
	code := `
import json
data = json.loads(body)
print(data.get("ip_addr"))
`
	out, err := executePython(context.Background(), code, input)
	if err != nil {
		t.Fatalf("executePython: %v", err)
	}
	if !strings.Contains(out, "54.89.198.138") {
		t.Fatalf("expected ip in stdout, got %q", out)
	}
}
