package imoutbound

import (
	"strings"
	"testing"
)

func TestChannelUserLabel(t *testing.T) {
	cases := map[string]string{
		"lark":     "飞书",
		"dingtalk": "钉钉",
		"qq":       "QQ",
		"telegram": "Telegram",
		"unknown":  "即时通讯",
		"":         "即时通讯",
	}
	for in, want := range cases {
		if got := ChannelUserLabel(in); got != want {
			t.Fatalf("ChannelUserLabel(%q)=%q want %q", in, got, want)
		}
	}
}

func TestImageGenerationUnavailableUserText_DingTalk(t *testing.T) {
	text := ImageGenerationUnavailableUserText("dingtalk")
	if !strings.Contains(text, "钉钉") || !strings.Contains(text, "不支持 AI 生成图片") {
		t.Fatalf("text=%q", text)
	}
}
