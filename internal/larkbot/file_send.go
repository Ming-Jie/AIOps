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
	"github.com/fisk086/aiops/internal/skills"
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

func (c *Client) uploadAndSendFile(ctx context.Context, cfg *BotConfig, target MessageTarget, absPath string) error {
	if !target.canSendAttachment() || cfg == nil {
		return fmt.Errorf("cannot send file: chat_id=%q open_id=%q", target.ChatID, target.OpenID)
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
		if err := skills.ValidateIMImageFile(absPath); err != nil {
			return fmt.Errorf("lark image validate: %w", err)
		}
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
		return c.sendMessageWithContent(ctx, cfg, target, "image", content)
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
	return c.sendMessageWithContent(ctx, cfg, target, "file", content)
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

func (c *Client) sendOutboundFiles(ctx context.Context, cfg *BotConfig, target MessageTarget, scope imoutbound.Scope, names []string) {
	if c.outbound == nil {
		c.outbound = imoutbound.GlobalStore()
	}
	if !target.canSendAttachment() {
		logger.Error("larkbot: cannot send attachments — missing chat_id and open_id", "files", names)
		return
	}
	const maxFilesPerReply = 5
	if len(names) > maxFilesPerReply {
		names = names[:maxFilesPerReply]
	}
	var missing []string
	for _, name := range names {
		abs, err := skills.ResolveIMAttachmentPath(c.outbound, scope, name)
		if err != nil {
			logger.Warn("larkbot: outbound file resolve failed", "file", name, "err", err)
			missing = append(missing, name)
			continue
		}
		if err := c.uploadAndSendFile(ctx, cfg, target, abs); err != nil {
			logger.Warn("larkbot: send file failed", "file", name, "err", err)
			missing = append(missing, name)
		} else {
			logger.Info("larkbot: file sent via create message", "file", name, "chat_id", target.ChatID)
		}
	}
	if len(missing) > 0 && target.canReplyText() {
		notice := fmt.Sprintf("以下附件未能发送（文件不存在或已过期）：%s", strings.Join(missing, ", "))
		if err := c.replyMessage(ctx, cfg, target.ReplyMessageID, notice); err != nil {
			logger.Warn("larkbot: attachment failure notice send failed", "err", err)
		}
	}
}
