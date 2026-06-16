package qqbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/skills"
)

const maxFilesPerReply = 5

type sendMsgRequest struct {
	Content  string        `json:"content"`
	MsgType  int           `json:"msg_type"`
	MsgID    string        `json:"msg_id,omitempty"`
	EventID  string        `json:"event_id,omitempty"`
	Media    *mediaContent `json:"media,omitempty"`
}

type mediaContent struct {
	FileInfo string `json:"file_info"`
}

type uploadFileResponse struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int    `json:"ttl"`
	ID       string `json:"id,omitempty"`
}

type fileType int

const (
	fileTypeImage fileType = 1
	fileTypeVideo fileType = 2
	fileTypeAudio fileType = 3
	fileTypeFile  fileType = 4
)

func detectFileType(name string) fileType {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return fileTypeImage
	case ".mp4", ".mov", ".avi", ".wmv", ".flv":
		return fileTypeVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".silk":
		return fileTypeAudio
	default:
		return fileTypeFile
	}
}

func (c *Client) sendPrivateMsg(ctx context.Context, appID, openUserID, text, msgID string) error {
	entry := c.getEntry(appID)
	if entry == nil {
		return fmt.Errorf("bot entry not found")
	}
	if err := entry.ensureToken(ctx); err != nil {
		return err
	}

	url := apiBase(entry.Config.Sandbox) + "/v2/users/" + openUserID + "/messages"
	payload := sendMsgRequest{
		Content: text,
		MsgType: 0,
		MsgID:   msgID,
	}
	return c.callAPI(ctx, url, payload, entry)
}

func (c *Client) sendGroupMsg(ctx context.Context, appID, groupOpenID, text, msgID string) error {
	entry := c.getEntry(appID)
	if entry == nil {
		return fmt.Errorf("bot entry not found")
	}
	if err := entry.ensureToken(ctx); err != nil {
		return err
	}

	url := apiBase(entry.Config.Sandbox) + "/v2/groups/" + groupOpenID + "/messages"
	payload := sendMsgRequest{
		Content: text,
		MsgType: 0,
		MsgID:   msgID,
	}
	return c.callAPI(ctx, url, payload, entry)
}

func (c *Client) sendMediaMsg(ctx context.Context, appID, targetID, fileInfo, msgType, msgID string) error {
	entry := c.getEntry(appID)
	if entry == nil {
		return fmt.Errorf("bot entry not found")
	}
	if err := entry.ensureToken(ctx); err != nil {
		return err
	}

	payload := sendMsgRequest{
		MsgType: 7,
		MsgID:   msgID,
		Media:   &mediaContent{FileInfo: fileInfo},
	}

	var url string
	if msgType == "group" {
		url = apiBase(entry.Config.Sandbox) + "/v2/groups/" + targetID + "/messages"
	} else {
		url = apiBase(entry.Config.Sandbox) + "/v2/users/" + targetID + "/messages"
	}
	return c.callAPI(ctx, url, payload, entry)
}

func (c *Client) uploadFile(ctx context.Context, appID, targetID, absPath, msgType string) (string, error) {
	entry := c.getEntry(appID)
	if entry == nil {
		return "", fmt.Errorf("bot entry not found")
	}
	if err := entry.ensureToken(ctx); err != nil {
		return "", err
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	ft := detectFileType(absPath)
	if msgType == "group" && ft == fileTypeFile {
		return "", fmt.Errorf("file type not supported in group chat")
	}

	var b64Buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &b64Buf)
	if _, err := io.Copy(encoder, f); err != nil {
		encoder.Close()
		return "", fmt.Errorf("encode file: %w", err)
	}
	encoder.Close()

	payload := map[string]any{
		"file_type":    int(ft),
		"file_data":    b64Buf.String(),
		"srv_send_msg": false,
	}

	var baseURL string
	if msgType == "group" {
		baseURL = apiBase(entry.Config.Sandbox) + "/v2/groups/" + targetID + "/files"
	} else {
		baseURL = apiBase(entry.Config.Sandbox) + "/v2/users/" + targetID + "/files"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal upload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", entry.authHeader())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var uploadResp uploadFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}

	if uploadResp.FileInfo == "" {
		return "", fmt.Errorf("upload response missing file_info")
	}

	return uploadResp.FileInfo, nil
}

func (c *Client) callAPI(ctx context.Context, url string, payload any, entry *BotEntry) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", entry.authHeader())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api %s status %d: %s", url, resp.StatusCode, truncate(string(respBody), 200))
	}

	return nil
}

func (c *Client) sendOutboundFiles(ctx context.Context, appID, senderID, groupOpenID string, scope imoutbound.Scope, names []string, msgType string) {
	store := imoutbound.GlobalStore()
	if len(names) > maxFilesPerReply {
		names = names[:maxFilesPerReply]
	}

	var targetID string
	if msgType == "group" {
		targetID = groupOpenID
	} else {
		targetID = senderID
	}

	for _, name := range names {
		abs, err := skills.ResolveIMAttachmentPath(store, scope, name)
		if err != nil {
			logger.Warn("qqbot: outbound file resolve failed", "file", name, "err", err)
			continue
		}

		fileInfo, err := c.uploadFile(ctx, appID, targetID, abs, msgType)
		if err != nil {
			logger.Warn("qqbot: file upload failed, sending as text", "file", name, "err", err)
			msg := fmt.Sprintf("文件：%s", name)
			if msgType == "group" && groupOpenID != "" {
				_ = c.sendGroupMsg(ctx, appID, groupOpenID, msg, "")
			} else {
				_ = c.sendPrivateMsg(ctx, appID, senderID, msg, "")
			}
			continue
		}

		if err := c.sendMediaMsg(ctx, appID, targetID, fileInfo, msgType, ""); err != nil {
			logger.Warn("qqbot: send media message failed", "file", name, "err", err)
			continue
		}
		logger.Info("qqbot: file sent as rich media", "file", name)
	}
}

func init() {
	// Ensure we have a reasonable timeout for HTTP calls
	http.DefaultClient.Timeout = 30 * time.Second
}
