package imoutbound

import "testing"

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
