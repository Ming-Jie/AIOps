package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fisk086/aiops/internal/imoutbound"
)

func TestFindRecentTerminalFilePath(t *testing.T) {
	dir, err := termFileStorageDir()
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "test-subdir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sub) })

	old := filepath.Join(sub, "old.log")
	newer := filepath.Join(sub, "report.log")
	if err := os.WriteFile(old, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(newer, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := FindRecentTerminalFilePath("report.log")
	if err != nil {
		t.Fatal(err)
	}
	if got != newer {
		t.Fatalf("got %q want %q", got, newer)
	}
}

func TestResolveIMAttachmentPath_TerminalFallback(t *testing.T) {
	dir, err := termFileStorageDir()
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "fallback-subdir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sub) })

	filename := "random_data_v2.log"
	full := filepath.Join(sub, filename)
	if err := os.WriteFile(full, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	store := &imoutbound.Store{}
	store.SetBase(t.TempDir())
	scope := imoutbound.Scope{AgentID: 1, SessionID: "missing-session"}

	got, err := ResolveIMAttachmentPath(store, scope, filename)
	if err != nil {
		t.Fatal(err)
	}
	if got != full {
		t.Fatalf("got %q want %q", got, full)
	}
}
