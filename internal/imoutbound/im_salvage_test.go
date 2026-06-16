package imoutbound

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func TestSalvageIMInlinePayload_Base64(t *testing.T) {
	dir := t.TempDir()
	store := &Store{}
	store.SetBase(dir)
	scope := Scope{AgentID: 1, SessionID: "sess-b64"}
	ctx := WithScope(context.Background(), 1, "sess-b64")
	payload := base64.StdEncoding.EncodeToString([]byte("hello im attachment"))
	reply := "已生成文件如下：\n" + payload

	name, ok := SalvageIMInlinePayload(ctx, store, scope, reply, "发我 random.txt")
	if !ok {
		t.Fatal("expected salvage")
	}
	if !strings.HasSuffix(name, ".txt") && !strings.HasSuffix(name, ".bin") {
		t.Fatalf("name=%q", name)
	}
	if len(AttachmentNamesForSend(ctx, nil)) == 0 {
		t.Fatal("expected staged file")
	}
}

func TestContainsSalvageableIMPayload_URLNotEnough(t *testing.T) {
	if ContainsSalvageableIMPayload("请下载 https://example.com/a.txt") {
		t.Fatal("plain URL should not be salvageable")
	}
}

func TestStripIMInlinePayload_RemovesBase64(t *testing.T) {
	b64 := strings.Repeat("QUJDRA==", 5) // 40 chars
	got := StripIMInlinePayload("说明如下\n" + b64 + "\n请查收")
	if strings.Contains(got, "QUJDRA") {
		t.Fatalf("base64 still present: %q", got)
	}
}
