package dingtalkbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/skills"
)

const maxDingtalkSendFileBytes = 20 << 20

var dingtalkImageExtensions = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".bmp": {},
}

func (c *Client) SetOutboundBase(dir string) {
	if c.outbound == nil {
		c.outbound = imoutbound.GlobalStore()
	}
	c.outbound.SetBase(dir)
}

func isDingtalkImageExt(ext string) bool {
	_, ok := dingtalkImageExtensions[strings.ToLower(ext)]
	return ok
}

func (c *Client) uploadMedia(ctx context.Context, cfg *BotConfig, absPath, mediaType string) (string, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.Size() <= 0 {
		return "", fmt.Errorf("empty file")
	}
	if info.Size() > maxDingtalkSendFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxDingtalkSendFileBytes)
	}

	oapiToken, err := c.tokens.getOAPIAccessToken(ctx, cfg.AppID, cfg.AppSecret)
	if err != nil {
		return "", err
	}

	uploadType := mediaType
	if uploadType == "video" {
		uploadType = "file"
	}
	url := fmt.Sprintf("%s/media/upload?access_token=%s&type=%s", dingtalkOAPIBase, oapiToken, uploadType)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("media", filepath.Base(absPath))
	if err != nil {
		return "", err
	}
	f, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("media upload decode: %w body=%s", err, string(raw))
	}
	if out.ErrCode != 0 || out.MediaID == "" {
		return "", fmt.Errorf("media upload errcode=%d msg=%s", out.ErrCode, out.ErrMsg)
	}
	return out.MediaID, nil
}

func (c *Client) sendSessionFile(ctx context.Context, cfg *BotConfig, sessionWebhook, absPath, mediaID string) error {
	fileName := filepath.Base(absPath)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if ext == "" {
		ext = "file"
	}
	body := map[string]any{
		"msgtype": "file",
		"file": map[string]any{
			"mediaId":  mediaID,
			"fileName": fileName,
			"fileType": ext,
		},
	}
	return c.postSessionWebhook(ctx, cfg, sessionWebhook, body)
}

func (c *Client) sendSessionImage(ctx context.Context, cfg *BotConfig, sessionWebhook, mediaID string) error {
	body := map[string]any{
		"msgtype": "image",
		"image": map[string]any{
			"mediaId": mediaID,
		},
	}
	return c.postSessionWebhook(ctx, cfg, sessionWebhook, body)
}

func (c *Client) uploadAndSendFile(ctx context.Context, cfg *BotConfig, sessionWebhook, absPath string) error {
	ext := filepath.Ext(absPath)
	mediaType := "file"
	if isDingtalkImageExt(ext) {
		if err := skills.ValidateIMImageFile(absPath); err != nil {
			return fmt.Errorf("dingtalk image validate: %w", err)
		}
		mediaType = "image"
	}
	mediaID, err := c.uploadMedia(ctx, cfg, absPath, mediaType)
	if err != nil {
		return err
	}
	if mediaType == "image" {
		if err := c.sendSessionImage(ctx, cfg, sessionWebhook, mediaID); err != nil {
			logger.Warn("dingtalkbot: image reply failed, fallback to file", "err", err)
			return c.sendSessionFile(ctx, cfg, sessionWebhook, absPath, mediaID)
		}
		return nil
	}
	return c.sendSessionFile(ctx, cfg, sessionWebhook, absPath, mediaID)
}

func (c *Client) sendOutboundFiles(ctx context.Context, cfg *BotConfig, sessionWebhook string, scope imoutbound.Scope, names []string) []string {
	if c.outbound == nil {
		c.outbound = imoutbound.GlobalStore()
	}
	const maxFilesPerReply = 5
	if len(names) > maxFilesPerReply {
		names = names[:maxFilesPerReply]
	}
	var failed []string
	for _, name := range names {
		abs, err := skills.ResolveIMAttachmentPath(c.outbound, scope, name)
		if err != nil {
			logger.Warn("dingtalkbot: outbound file resolve failed", "file", name, "err", err)
			failed = append(failed, name)
			continue
		}
		if err := c.uploadAndSendFile(ctx, cfg, sessionWebhook, abs); err != nil {
			logger.Warn("dingtalkbot: send file failed", "file", name, "err", err)
			failed = append(failed, name)
		} else {
			logger.Info("dingtalkbot: file sent", "file", name)
		}
	}
	return failed
}
