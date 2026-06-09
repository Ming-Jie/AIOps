package openviking

import "testing"

func TestDisplayNameFromURI (t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"viking://resources/kb/1/5_Vault.pdf/Vault/部署VaultHelmInstall.md", "部署VaultHelmInstall.md"},
		{"viking://resources/kb/1/5_filename.pdf", "filename.pdf"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := DisplayNameFromURI(tc.uri); got != tc.want {
			t.Fatalf("DisplayNameFromURI(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

func TestTruncateSnippet (t *testing.T) {
	if got := TruncateSnippet("hello", 10); got != "hello" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := TruncateSnippet("1234567890", 5); got != "12345…" {
		t.Fatalf("unexpected: %q", got)
	}
}
