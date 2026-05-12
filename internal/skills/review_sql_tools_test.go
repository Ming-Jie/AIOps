package skills

import (
	"context"
	"strings"
	"testing"
)

func TestExecBuiltinCodeReviewFlagsSecurityIssues(t *testing.T) {
	out, err := execBuiltinCodeReview(context.Background(), map[string]any{
		"code": `db.Query(fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email))
password := "supersecret123"
http.Get("http://example.com")`,
	})
	if err != nil {
		t.Fatalf("execBuiltinCodeReview returned error: %v", err)
	}
	for _, want := range []string{"sql_injection", "secret_password_assignment", "plaintext_http"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestExecBuiltinCodeReviewSecurityFocusLimitsNonSecurityFindings(t *testing.T) {
	out, err := execBuiltinCodeReview(context.Background(), map[string]any{
		"code": `panic("boom")
http.Get("http://example.com")`,
		"focus": "security",
	})
	if err != nil {
		t.Fatalf("execBuiltinCodeReview returned error: %v", err)
	}
	if !strings.Contains(out, "plaintext_http") {
		t.Fatalf("expected plaintext_http in output, got:\n%s", out)
	}
	if strings.Contains(out, "panic_or_exit") {
		t.Fatalf("security focus should not include panic_or_exit, got:\n%s", out)
	}
}

func TestExecBuiltinSQLExplainAnalyzesSeqScan(t *testing.T) {
	out, err := execBuiltinSQLExplain(context.Background(), map[string]any{
		"plan": `Seq Scan on users  (cost=0.00..431.00 rows=10000 width=64)
  Filter: (email = 'a@example.com')
  Rows Removed by Filter: 9990`,
		"engine": "postgres",
		"query":  "SELECT * FROM users WHERE email = 'a@example.com'",
	})
	if err != nil {
		t.Fatalf("execBuiltinSQLExplain returned error: %v", err)
	}
	for _, want := range []string{"Engine: postgres", "seq_scan", "wide_select"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}
