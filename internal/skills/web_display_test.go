package skills

import (
	"strings"
	"testing"
)

func TestSanitizeWebAssistantText(t *testing.T) {
	in := "已为您生成 random_data.txt，请使用下方按钮下载。\n\nRandom file generated: random_file.txt\nN0nkNTMizdnDO4Xl5//9/vEe4by3AcDM3wFSaJljWmZM+L27PWoTFDRTIgikqtHetA15GW5qBjroI32rjhbfiQa6EtUt2ByfsRxRzFuatJG/0jmId7FFyYMt91mt13gw7HQCtA==\n\n文件已准备好。"
	got := SanitizeWebAssistantText(in)
	want := "已为您生成 random_data.txt，请使用下方按钮下载。\n文件已准备好。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSanitizeWebAssistantText_TruncatesHugeContent(t *testing.T) {
	var b strings.Builder
	b.WriteString("处理完成，请下载附件。\n\n")
	for i := 0; i < MaxWebBubbleRunes+500; i++ {
		b.WriteRune('数')
	}
	got := SanitizeWebAssistantText(b.String())
	if len([]rune(got)) > MaxWebBubbleRunes+80 {
		t.Fatalf("expected truncation, got %d runes", len([]rune(got)))
	}
	if !strings.Contains(got, "内容过长") {
		preview := got
		if len(preview) > 200 {
			preview = preview[:200]
		}
		t.Fatalf("missing truncation notice: %q", preview)
	}
}

func TestSanitizeWebAssistantText_StripsPastedFileBody(t *testing.T) {
	in := `已保存至 chuntian.txt。chuntian.txt

春意
春风裁绿柳，细雨洗尘埃。
桃红凝晓露，莺啭唤花开。

请点击下载。`
	got := SanitizeWebAssistantText(in)
	if strings.Contains(got, "春风裁绿柳") {
		t.Fatalf("poem body should be stripped: %q", got)
	}
	if !strings.Contains(got, "请点击下载") {
		t.Fatalf("prose removed: %q", got)
	}
}

func TestSanitizeWebAssistantText_DataOnlyFallsBack(t *testing.T) {
	in := "7821, 3492, 1056, 9234, 5567\n2189, 6743, 8812"
	got := SanitizeWebAssistantText(in)
	if got != WebContentOmittedFallback {
		t.Fatalf("got %q want fallback", got)
	}
}
