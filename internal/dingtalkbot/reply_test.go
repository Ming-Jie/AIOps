package dingtalkbot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseDingtalkResponse(t *testing.T) {
	if err := parseDingtalkResponse(200, []byte(`{"errcode":0,"errmsg":"ok"}`)); err != nil {
		t.Fatalf("expected ok: %v", err)
	}
	if err := parseDingtalkResponse(200, []byte(`{"errcode":40014,"errmsg":"invalid token"}`)); err == nil {
		t.Fatal("expected errcode error")
	}
	if err := parseDingtalkResponse(200, []byte(`{"success":false,"message":"fail"}`)); err == nil {
		t.Fatal("expected success=false error")
	}
}

func TestSplitReplyChunks(t *testing.T) {
	short := "hello"
	if chunks := splitReplyChunks(short, false); len(chunks) != 1 || chunks[0] != short {
		t.Fatalf("short=%v", chunks)
	}
	long := strings.Repeat("字", maxTextChunkRunes+100)
	chunks := splitReplyChunks(long, false)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	total := 0
	for _, c := range chunks {
		total += utf8.RuneCountInString(c)
	}
	if total != utf8.RuneCountInString(long) {
		t.Fatalf("rune count mismatch %d vs %d", total, utf8.RuneCountInString(long))
	}
}
