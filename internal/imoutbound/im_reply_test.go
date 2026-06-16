package imoutbound

import (
	"context"
	"strings"
	"testing"
)

func TestAttachmentNamesForSend_PrefersStaged(t *testing.T) {
	ctx := WithScope(context.Background(), 1, "sess-1")
	RegisterWrittenFile(ctx, "random_data.txt")

	got := AttachmentNamesForSend(ctx, []string{"random_data.txt", "fake.txt"})
	if len(got) != 1 || got[0] != "random_data.txt" {
		t.Fatalf("got %v", got)
	}
}

func TestSanitizeIMReplyText(t *testing.T) {
	dir := t.TempDir()
	store := &Store{}
	store.SetBase(dir)
	scope := Scope{AgentID: 1, SessionID: "sess-1"}
	_, _ = store.WriteFile(scope, "random_data.txt", "aK9zL2mP5qR8vW4xY1bZ")

	in := "好的，为你生成了一个包含 20 个随机字符的文本文件 random_data.txt。\n\nrandom_data.txt\n\n文件内容如下：aK9zL2mP5qR8vW4xY1bZ\n\n需要调整吗？"
	got := SanitizeIMReplyText(in, scope, store, []string{"random_data.txt"})
	if strings.Contains(got, "aK9zL2mP5qR8vW4xY1bZ") {
		t.Fatalf("file body still present: %q", got)
	}
	if strings.Contains(got, "文件内容如下") {
		t.Fatalf("intro still present: %q", got)
	}
	if !strings.Contains(got, "需要调整吗？") {
		t.Fatalf("prose removed: %q", got)
	}
}
