package dingtalkbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/notify"
)

var (
	reMarkdownBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reMarkdownHeader = regexp.MustCompile(`(?m)^#{1,6}\s*`)
)

const replyPhaseTimeout = 90 * time.Second

func needsRichReply(text string) bool {
	if utf8.RuneCountInString(text) > 400 {
		return true
	}
	if strings.Count(text, "\n") >= 3 {
		return true
	}
	for _, hint := range []string{"**", "###", "## ", "# ", "\n* ", "\n- ", "```", "---"} {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}

func normalizeBotReplyMarkdown(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			continue
		}
		if strings.HasPrefix(trimmed, "* ") {
			prefix := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			line = prefix + "- " + strings.TrimPrefix(trimmed, "* ")
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func plainTextForDingtalk(s string) string {
	s = normalizeBotReplyMarkdown(s)
	s = reMarkdownBold.ReplaceAllString(s, "$1")
	s = reMarkdownHeader.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func (c *Client) replySessionMessage(ctx context.Context, cfg *BotConfig, sessionWebhook, text string) error {
	sessionWebhook = strings.TrimSpace(sessionWebhook)
	text = strings.TrimSpace(text)
	if sessionWebhook == "" || text == "" || cfg == nil {
		return nil
	}

	chunks := splitReplyChunks(text, needsRichReply(text))
	var firstErr error
	for i, chunk := range chunks {
		if err := c.replySessionMessageOnce(ctx, cfg, sessionWebhook, chunk, i+1, len(chunks)); err != nil {
			logger.Warn("dingtalkbot: reply chunk failed", "chunk", i+1, "total", len(chunks), "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (c *Client) replySessionMessageOnce(ctx context.Context, cfg *BotConfig, sessionWebhook, text string, idx, total int) error {
	var body map[string]any
	if needsRichReply(text) {
		md := normalizeBotReplyMarkdown(text)
		title, bodyText := notify.LarkCardTitleAndBody(md)
		if strings.TrimSpace(bodyText) == "" {
			bodyText = md
		}
		if total > 1 {
			title = fmt.Sprintf("%s (%d/%d)", title, idx, total)
		}
		body = map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]any{
				"title": title,
				"text":  bodyText,
			},
		}
		err := c.postSessionWebhook(ctx, cfg, sessionWebhook, body)
		if err != nil {
			logger.Warn("dingtalkbot: markdown chunk failed, fallback to plain text", "chunk", idx, "err", err)
			body = map[string]any{
				"msgtype": "text",
				"text": map[string]any{
					"content": plainTextForDingtalk(text),
				},
			}
			return c.postSessionWebhook(ctx, cfg, sessionWebhook, body)
		}
		return nil
	}

	content := text
	if total > 1 {
		content = fmt.Sprintf("(%d/%d)\n%s", idx, total, text)
	}
	body = map[string]any{
		"msgtype": "text",
		"text": map[string]any{
			"content": content,
		},
	}
	return c.postSessionWebhook(ctx, cfg, sessionWebhook, body)
}

func splitReplyChunks(text string, rich bool) []string {
	limit := maxTextChunkRunes
	if rich {
		limit = maxMarkdownChunkRunes
	}
	if utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}
	// Prefer splitting on paragraph boundaries.
	parts := strings.Split(text, "\n\n")
	var chunks []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		chunks = append(chunks, strings.TrimSpace(buf.String()))
		buf.Reset()
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		sep := ""
		if buf.Len() > 0 {
			sep = "\n\n"
		}
		candidate := buf.String() + sep + p
		if utf8.RuneCountInString(candidate) <= limit {
			if buf.Len() > 0 {
				buf.WriteString("\n\n")
			}
			buf.WriteString(p)
			continue
		}
		flush()
		if utf8.RuneCountInString(p) <= limit {
			buf.WriteString(p)
			continue
		}
		// Hard-split an oversized paragraph by runes.
		for _, sub := range splitRunes(p, limit) {
			chunks = append(chunks, sub)
		}
	}
	flush()
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}

func splitRunes(s string, limit int) []string {
	if limit <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	var out []string
	for i := 0; i < len(runes); i += limit {
		end := i + limit
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}

func (c *Client) postSessionWebhook(ctx context.Context, cfg *BotConfig, sessionWebhook string, body map[string]any) error {
	token, err := c.tokens.getAPIAccessToken(ctx, cfg.AppID, cfg.AppSecret)
	if err != nil {
		return fmt.Errorf("dingtalk reply token: %w", err)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sessionWebhook, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return parseDingtalkResponse(resp.StatusCode, respBody)
}

func parseDingtalkResponse(statusCode int, body []byte) error {
	if statusCode != http.StatusOK {
		return fmt.Errorf("http %d: %s", statusCode, string(body))
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		return nil
	}
	if success, ok := generic["success"].(bool); ok && !success {
		return fmt.Errorf("api success=false: %s", string(body))
	}
	if code, ok := generic["errcode"].(float64); ok && code != 0 {
		msg, _ := generic["errmsg"].(string)
		return fmt.Errorf("api errcode=%.0f msg=%s", code, msg)
	}
	if code, ok := generic["code"].(string); ok && code != "" && code != "0" && code != "OK" {
		return fmt.Errorf("api code=%s: %s", code, string(body))
	}
	return nil
}

func replyContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent != nil && parent.Err() == nil {
		return context.WithTimeout(parent, replyPhaseTimeout)
	}
	return context.WithTimeout(context.Background(), replyPhaseTimeout)
}
