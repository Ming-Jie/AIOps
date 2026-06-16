package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectNewFiles_IgnoresUnchangedAndFindsSlowCommandOutput(t *testing.T) {
	dir := t.TempDir()
	unchanged := filepath.Join(dir, "unchanged.txt")
	if err := os.WriteFile(unchanged, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	before := snapshotDir(dir)

	slow := filepath.Join(dir, "random_sample.txt")
	// Simulate a file created early in a long-running command (>5s ago).
	past := time.Now().Add(-10 * time.Second)
	if err := os.WriteFile(slow, []byte("sample"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(slow, past, past); err != nil {
		t.Fatal(err)
	}

	attachments := detectNewFiles(context.Background(), dir, before)
	if len(attachments) != 1 {
		t.Fatalf("attachments=%d want 1", len(attachments))
	}
	if attachments[0].Filename != "random_sample.txt" {
		t.Fatalf("filename=%s", attachments[0].Filename)
	}
}

func TestDetectNewFiles_AcceptsBinExtension(t *testing.T) {
	dir := t.TempDir()
	before := snapshotDir(dir)

	binPath := filepath.Join(dir, "random_file.bin")
	if err := os.WriteFile(binPath, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	attachments := detectNewFiles(context.Background(), dir, before)
	if len(attachments) != 1 {
		t.Fatalf("attachments=%d want 1", len(attachments))
	}
	if attachments[0].Filename != "random_file.bin" {
		t.Fatalf("filename=%s", attachments[0].Filename)
	}
	if attachments[0].MimeType != "application/octet-stream" {
		t.Fatalf("mime=%s", attachments[0].MimeType)
	}
}

func TestDetectNewFiles_DetectsOverwriteBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "random_file.bin")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	before := snapshotDir(dir)

	if err := os.WriteFile(path, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}
	// Keep same modtime as before to ensure size-based detection works.
	if err := os.Chtimes(path, before["random_file.bin"].mod, before["random_file.bin"].mod); err != nil {
		t.Fatal(err)
	}

	attachments := detectNewFiles(context.Background(), dir, before)
	if len(attachments) != 1 {
		t.Fatalf("attachments=%d want 1", len(attachments))
	}
}

func TestMimeForFilename_UnknownExt(t *testing.T) {
	if got := mimeForFilename("data.unknownext"); got != defaultBinaryMIME {
		t.Fatalf("got %q", got)
	}
}
