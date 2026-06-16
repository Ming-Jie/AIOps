package imoutbound

import (
	"strings"
	"testing"
)

func TestParseFileMarkers(t *testing.T) {
	text := "这是说明。\n[[lark_file:report.txt]]\n[[lark_file:report.txt]]\n[[dingtalk_file:chart.png]]"
	clean, files := ParseFileMarkers(text)
	if clean != "这是说明。" {
		t.Fatalf("clean=%q", clean)
	}
	if len(files) != 2 || files[0] != "report.txt" || files[1] != "chart.png" {
		t.Fatalf("files=%v", files)
	}
}

func TestContentForSummary(t *testing.T) {
	in := "截图完成。[[im_file:shot.png]]"
	out := ContentForSummary(in)
	if contains(out, "[[im_file:") {
		t.Fatalf("marker should be replaced in summary input: %q", out)
	}
	if !contains(out, "[附件: shot.png]") {
		t.Fatalf("expected attachment note: %q", out)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || strings.Contains(s, sub)
}
