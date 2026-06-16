package imoutbound

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPartitionRegisteredFiles(t *testing.T) {
	ctx := WithScope(context.Background(), 1, "sess-a")
	RegisterWrittenFile(ctx, "ok.txt")

	reg, unreg := PartitionRegisteredFiles(ctx, []string{"ok.txt", "fake.txt"})
	if len(reg) != 1 || reg[0] != "ok.txt" {
		t.Fatalf("registered=%v", reg)
	}
	if len(unreg) != 1 || unreg[0] != "fake.txt" {
		t.Fatalf("unregistered=%v", unreg)
	}
}

func TestFindRecentFileForAgent(t *testing.T) {
	dir := t.TempDir()
	store := &Store{}
	store.SetBase(dir)

	oldSess := filepath.Join(dir, "lark-outbound", "1", "old-session")
	newSess := filepath.Join(dir, "lark-outbound", "1", "new-session")
	for _, sess := range []string{oldSess, newSess} {
		if err := os.MkdirAll(sess, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	oldFile := filepath.Join(oldSess, "random_sample.txt")
	newFile := filepath.Join(newSess, "random_sample.txt")
	if err := os.WriteFile(oldFile, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(newFile, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}

	got, err := store.FindRecentFileForAgent(1, "random_sample.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != newFile {
		t.Fatalf("got %q want %q", got, newFile)
	}
}
