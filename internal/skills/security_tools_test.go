package skills

import (
	"context"
	"strings"
	"testing"
)

func TestExecBuiltinSecretsScannerDetectsPasswordColonAssignment(t *testing.T) {
	out, err := execBuiltinSecretsScanner(context.Background(), map[string]any{
		"text": `password: "topsecret123"`,
	})
	if err != nil {
		t.Fatalf("execBuiltinSecretsScanner returned error: %v", err)
	}
	if !strings.Contains(out, "password_assignment") {
		t.Fatalf("expected password_assignment in output, got:\n%s", out)
	}
}
