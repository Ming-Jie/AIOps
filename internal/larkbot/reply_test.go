package larkbot

import (
	"strings"
	"testing"
)

func TestNeedsRichReply(t *testing.T) {
	if needsRichReply("11") {
		t.Fatal("short plain text should not need rich reply")
	}
	if !needsRichReply("**bold**\nline2") {
		t.Fatal("markdown should need rich reply")
	}
	if !needsRichReply("a\nb\nc\nd") {
		t.Fatal("multi-line should need rich reply")
	}
}

func TestNormalizeBotReplyMarkdown(t *testing.T) {
	in := "### Title\n* item one\n---\n* item two"
	out := normalizeBotReplyMarkdown(in)
	if strings.Contains(out, "---") {
		t.Fatalf("should drop HR: %q", out)
	}
	if !strings.Contains(out, "- item one") {
		t.Fatalf("should convert bullets: %q", out)
	}
}
