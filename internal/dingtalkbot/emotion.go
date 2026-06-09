package dingtalkbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fisk086/aiops/internal/logger"
)

// addAckEmotion posts a standard emoji reaction on the user's message (aligns with Lark "Get" ack).
func (c *Client) addAckEmotion(ctx context.Context, cfg *BotConfig, openMsgID, openConversationID string) {
	openMsgID = strings.TrimSpace(openMsgID)
	openConversationID = strings.TrimSpace(openConversationID)
	if cfg == nil || openMsgID == "" || openConversationID == "" {
		return
	}
	for _, emotionName := range []string{DefaultAckEmotionName, "RogerThat"} {
		body := map[string]any{
			"robotCode":          cfg.AppID,
			"openMsgId":          openMsgID,
			"openConversationId": openConversationID,
			"emotionType":        1,
			"emotionName":        emotionName,
		}
		if err := c.postDingtalkAPI(ctx, cfg, "/v1.0/robot/emotion/reply", body); err != nil {
			logger.Warn("dingtalkbot: ack emotion failed", "msg_id", openMsgID, "emotion", emotionName, "err", err)
			continue
		}
		logger.Info("dingtalkbot: ack emotion sent", "msg_id", openMsgID, "emotion", emotionName)
		return
	}
}

func (c *Client) postDingtalkAPI(ctx context.Context, cfg *BotConfig, path string, body map[string]any) error {
	token, err := c.tokens.getAPIAccessToken(ctx, cfg.AppID, cfg.AppSecret)
	if err != nil {
		return fmt.Errorf("access token: %w", err)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := dingtalkAPIBase + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return parseDingtalkResponse(resp.StatusCode, respBody)
}
