package kbimport

import (
	"testing"
)

func TestValidateURL (t *testing.T) {
	tests := []struct {
		raw string
		ok  bool
	}{
		{"https://example.com/doc.pdf", true},
		{"http://example.com/a.pdf", false},
		{"https://user:pass@example.com/a.pdf", false},
		{"https://127.0.0.1/a.pdf", false},
		{"https://localhost/a.pdf", false},
		{"", false},
		{"ftp://example.com/a.pdf", false},
	}
	for _, tc := range tests {
		_, err := ValidateURL(tc.raw)
		if tc.ok && err != nil {
			t.Fatalf("ValidateURL(%q) unexpected err: %v", tc.raw, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("ValidateURL(%q) expected error", tc.raw)
		}
	}
}

func TestInferFilename (t *testing.T) {
	u, err := ValidateURL("https://example.com/path/report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	name := InferFilename(u, "", "application/pdf", "")
	if name != "report.pdf" {
		t.Fatalf("got %q", name)
	}
	name = InferFilename(u, "my doc", "application/pdf", "")
	if name != "my doc.pdf" {
		t.Fatalf("got %q", name)
	}
}

func TestSanitizeFilename (t *testing.T) {
	if got := sanitizeFilename("../../etc/passwd"); got != "passwd" {
		t.Fatalf("got %q", got)
	}
}
