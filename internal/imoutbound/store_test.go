package imoutbound

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndResolveFile(t *testing.T) {
	dir := t.TempDir()
	store := &Store{}
	store.SetBase(dir)
	scope := Scope{AgentID: 1, SessionID: "sess-abc"}
	marker, err := store.WriteFile(scope, "hello.txt", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if marker != "[[lark_file:hello.txt]]" {
		t.Fatalf("marker=%s", marker)
	}
	path, err := store.ResolveFile(scope, "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hi" {
		t.Fatalf("data=%q", data)
	}
	wantDir := filepath.Join(dir, "lark-outbound", "1", "sess-abc")
	if filepath.Dir(path) != wantDir {
		t.Fatalf("dir=%s want=%s", filepath.Dir(path), wantDir)
	}
}
