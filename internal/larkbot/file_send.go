package larkbot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

const maxLarkSendFileBytes = 25 << 20 // Lark IM file limit ~30MB; stay conservative

var imageExtensions = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".bmp": {},
}

func (c *Client) SetOutboundBase(dir string) {
	if c.outbound == nil {
		c.outbound = imoutbound.GlobalStore()
	}
	c.outbound.SetBase(dir)
}

func larkFileTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".pdf":
		return larkim.CreateFileFileTypePdf
	case ".doc":
		return larkim.CreateFileFileTypeDoc
	case ".xls":
		return larkim.CreateFileFileTypeXls
	case ".ppt":
		return larkim.CreateFileFileTypePpt
	case ".mp4":
		return larkim.CreateFileFileTypeMp4
	case ".opus":
		return larkim.CreateFileFileTypeOpus
	default:
		return larkim.CreateFileFileTypeStream
	}
}

func isImageExt(ext string) bool {
	_, ok := imageExtensions[strings.ToLower(ext)]
	return ok
}

func (c *Client) uploadAndReplyFile(ctx context.Context, cfg *BotConfig, messageID, absPath string) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" || cfg == nil {
		return nil
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.Size() <= 0 {
		return fmt.Errorf("empty file")
	}
	if info.Size() > maxLarkSendFileBytes {
		return fmt.Errorf("file exceeds %d bytes", maxLarkSendFileBytes)
	}

	ext := filepath.Ext(absPath)
	fileName := filepath.Base(absPath)
	api := newLarkAPIClient(cfg)

	if isImageExt(ext) {
		body, err := larkim.NewCreateImagePathReqBodyBuilder().
			ImageType(larkim.CreateImageImageTypeMessage).
			ImagePath(absPath).
			Build()
		if err != nil {
			return fmt.Errorf("build image upload: %w", err)
		}
		resp, err := api.Im.Image.Create(ctx, larkim.NewCreateImageReqBuilder().Body(body).Build())
		if err != nil {
			return fmt.Errorf("lark image upload: %w", err)
		}
		if resp == nil || !resp.Success() || resp.Data == nil || resp.Data.ImageKey == nil {
			code, msg := 0, "unknown"
			if resp != nil {
				code, msg = resp.Code, resp.Msg
			}
			return fmt.Errorf("lark image upload code=%d msg=%s", code, msg)
		}
		content, err := larkImageContent(*resp.Data.ImageKey)
		if err != nil {
			return err
		}
		return c.replyWithContent(ctx, cfg, messageID, "image", content)
	}

	body, err := larkim.NewCreateFilePathReqBodyBuilder().
		FileType(larkFileTypeFromExt(ext)).
		FileName(fileName).
		FilePath(absPath).
		Build()
	if err != nil {
		return fmt.Errorf("build file upload: %w", err)
	}
	resp, err := api.Im.File.Create(ctx, larkim.NewCreateFileReqBuilder().Body(body).Build())
	if err != nil {
		return fmt.Errorf("lark file upload: %w", err)
	}
	if resp == nil || !resp.Success() || resp.Data == nil || resp.Data.FileKey == nil {
		code, msg := 0, "unknown"
		if resp != nil {
			code, msg = resp.Code, resp.Msg
		}
		return fmt.Errorf("lark file upload code=%d msg=%s", code, msg)
	}
	content, err := larkFileContent(*resp.Data.FileKey)
	if err != nil {
		return err
	}
	return c.replyWithContent(ctx, cfg, messageID, "file", content)
}

func larkFileContent(fileKey string) (string, error) {
	b, err := json.Marshal(map[string]string{"file_key": fileKey})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func larkImageContent(imageKey string) (string, error) {
	b, err := json.Marshal(map[string]string{"image_key": imageKey})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) sendOutboundFiles(ctx context.Context, cfg *BotConfig, messageID string, scope imoutbound.Scope, names []string) {
	if c.outbound == nil {
		c.outbound = imoutbound.GlobalStore()
	}
	const maxFilesPerReply = 5
	if len(names) > maxFilesPerReply {
		names = names[:maxFilesPerReply]
	}
	for _, name := range names {
		abs, err := c.outbound.ResolveFile(scope, name)
		if err != nil {
			logger.Warn("larkbot: outbound file resolve failed", "file", name, "err", err)
			continue
		}
		if err := c.uploadAndReplyFile(ctx, cfg, messageID, abs); err != nil {
			logger.Warn("larkbot: send file failed", "file", name, "err", err)
		} else {
			logger.Info("larkbot: file sent", "file", name, "message_id", messageID)
		}
	}
}

func larkFileSendSystemHint() string {
	return `（系统提示）当前为飞书 IM 对话。若用户需要可下载/保存的文件，请使用工具 builtin_im_save_file 写入文件，并在最终回复中**原样保留**工具返回的 [[lark_file:文件名]] 标记（可多个）。机器人会自动上传并以文件/图片消息发送。纯文本说明可与标记同条回复。`
}
