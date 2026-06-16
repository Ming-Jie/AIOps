package imoutbound

import (
	"strings"
	"testing"
)

func TestStripIMExternalURLs(t *testing.T) {
	in := "请访问 https://10.0.0.1:8080/api/v1/chat/files/chat-generated/1/a.txt 或 /api/v1/chat/terminal-files/x/y.bin 下载"
	got := StripIMExternalURLs(in)
	if strings.Contains(got, "http") || strings.Contains(got, "/api/v1/chat/") {
		t.Fatalf("URLs not stripped: %q", got)
	}
}

func TestSanitizeNotifyText(t *testing.T) {
	in := "结论如下\n\ndata:image/png;base64,abc123\n\n详见 /api/v1/chat/files/x/y.txt"
	got := SanitizeNotifyText(in)
	if got == in {
		t.Fatalf("unchanged: %q", got)
	}
}

func TestExtractPastedIMFile_UserExample(t *testing.T) {
	in := `好的，这次为你生成了一组 20 个 1000 到 9999 之间的随机数字，已保存至 random_numbers_v2.txt。random_numbers_v2.txt

7821, 3492, 1056, 9234, 5567,
2189, 6743, 8812, 4409, 3921,
5032, 9987, 1245, 6634, 7710,
2843, 4156, 8390, 5678, 3021

如果需要特定的格式（比如 CSV 或一行一个），或者数量更多，随时跟我说！`

	name, body, ok := ExtractPastedIMFile(in)
	if !ok {
		t.Fatal("expected extraction")
	}
	if name != "random_numbers_v2.txt" {
		t.Fatalf("name=%q", name)
	}
	if !strings.Contains(body, "7821") || !strings.Contains(body, "3021") {
		t.Fatalf("body=%q", body)
	}
	if strings.Contains(body, "如果需要") {
		t.Fatalf("closing prose leaked into body: %q", body)
	}
}

func TestStripPastedFileBody_UserExample(t *testing.T) {
	in := `好的，这次为你生成了一组 20 个 1000 到 9999 之间的随机数字，已保存至 random_numbers_v2.txt。random_numbers_v2.txt

7821, 3492, 1056`

	got := StripPastedFileBody(in)
	if strings.Contains(got, "7821") {
		t.Fatalf("numbers still in text: %q", got)
	}
	if !strings.Contains(got, "随机数字") {
		t.Fatalf("prose removed: %q", got)
	}
}

func TestShortIMAttachmentReply(t *testing.T) {
	got := ShortIMAttachmentReply("random_numbers_v2.txt")
	if got != "已生成 random_numbers_v2.txt，请查收附件。" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeIMReplyText_StripsURLs(t *testing.T) {
	in := "已生成文件，下载链接 https://app.example.com/api/v1/chat/files/x/y.txt"
	got := SanitizeIMReplyText(in, Scope{}, nil, nil)
	if got == in || strings.Contains(got, "http") {
		t.Fatalf("URLs not stripped: %q", got)
	}
}
