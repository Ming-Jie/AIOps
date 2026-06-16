package imoutbound

import (
	"context"
	"strings"
	"testing"
)

func TestDeliverIMReply_ImageGenerationRequest_NoGenericFailureSuffix(t *testing.T) {
	ctx := context.Background()
	out := DeliverIMReply(DeliverInput{
		Channel:     "lark",
		Ctx:         ctx,
		AgentCtx:    ctx,
		UserRequest: "帮我生成一个小猫的图片发给我",
		AgentText:   "暂时无法生成图片，你可以发一张图让我处理，或让我截图指定网页。",
	})
	if len(out.FileNames) != 0 {
		t.Fatalf("files=%v", out.FileNames)
	}
	want := ImageGenerationUnavailableUserText("lark")
	if out.Text != want {
		t.Fatalf("text=%q want %q", out.Text, want)
	}
	if !strings.Contains(out.Text, "飞书") {
		t.Fatalf("lark channel should mention 飞书: %q", out.Text)
	}
	if strings.Contains(out.Text, "附件未能发送") {
		t.Fatalf("should not append generic file failure notice: %q", out.Text)
	}
}

func TestDeliverIMReply_TextFileRequest_StillGetsFailureNotice(t *testing.T) {
	ctx := context.Background()
	out := DeliverIMReply(DeliverInput{
		Channel:     "lark",
		Ctx:         ctx,
		AgentCtx:    ctx,
		UserRequest: "帮我生成一个 random.txt 发给我",
		AgentText:   "文件已保存，请点击下载 https://example.com/api/v1/chat/files/x/random.txt",
	})
	if strings.Contains(out.Text, "不支持 AI 生成图片") {
		t.Fatalf("text file request should not get image message: %q", out.Text)
	}
	if !strings.Contains(out.Text, "附件未能") {
		t.Fatalf("expected attachment failure notice for text file delivery miss: %q", out.Text)
	}
}
