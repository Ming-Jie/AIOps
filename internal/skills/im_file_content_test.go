package skills

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestPrepareIMFileBytes_PlainAndBase64(t *testing.T) {
	plain, err := prepareIMFileBytes("a.txt", "hello")
	if err != nil || !bytes.Equal(plain, []byte("hello")) {
		t.Fatalf("plain: %v %q", err, plain)
	}
	raw := base64.StdEncoding.EncodeToString([]byte("bin-data"))
	got, err := prepareIMFileBytes("a.bin", raw)
	if err != nil || !bytes.Equal(got, []byte("bin-data")) {
		t.Fatalf("b64: %v %q", err, got)
	}
}

func TestPrepareIMFileBytes_RejectsPlaceholderPNG(t *testing.T) {
	// Classic 1×1 PNG — LLMs often fabricate this for "cat.png" etc.
	b64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	_, err := prepareIMFileBytes("cat.png", b64)
	if err == nil {
		t.Fatal("expected rejection for 1x1 placeholder png")
	}
}
