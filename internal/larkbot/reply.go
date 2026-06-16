package larkbot

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/notify"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

var (
	reMarkdownBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reMarkdownHeader = regexp.MustCompile(`(?m)^#{1,6}\s*`)
)

func larkTextContent(text string) (string, error) {
	b, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func newLarkAPIClient(cfg *BotConfig) *lark.Client {
	domain := openAPIDomainFromBot(cfg)
	return lark.NewClient(cfg.AppID, cfg.AppSecret, lark.WithOpenBaseUrl(domain))
}

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

func plainTextForLark(s string) string {
	s = normalizeBotReplyMarkdown(s)
	s = reMarkdownBold.ReplaceAllString(s, "$1")
	s = reMarkdownHeader.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func (c *Client) replyMessage(ctx context.Context, cfg *BotConfig, messageID, text string) error {
	messageID = strings.TrimSpace(messageID)
	text = strings.TrimSpace(text)
	if messageID == "" || text == "" {
		return nil
	}
	if cfg == nil || cfg.AppID == "" || cfg.AppSecret == "" {
		return fmt.Errorf("lark reply: missing bot credentials")
	}

	if !needsRichReply(text) {
		return c.replyWithContent(ctx, cfg, messageID, "text", mustLarkTextContent(text))
	}

	md := normalizeBotReplyMarkdown(text)
	title, body := notify.LarkCardTitleAndBody(md)
	if strings.TrimSpace(body) == "" {
		body = md
	}
	content := notify.LarkInteractiveMessageContent(title, body)
	if err := c.replyWithContent(ctx, cfg, messageID, "interactive", content); err != nil {
		logger.Warn("larkbot: interactive reply failed, fallback to plain text", "message_id", messageID, "err", err)
		return c.replyWithContent(ctx, cfg, messageID, "text", mustLarkTextContent(plainTextForLark(text)))
	}
	return nil
}

func mustLarkTextContent(text string) string {
	content, err := larkTextContent(text)
	if err != nil {
		return `{"text":""}`
	}
	return content
}

func (c *Client) replyWithContent(ctx context.Context, cfg *BotConfig, messageID, msgType, content string) error {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := newLarkAPIClient(cfg).Im.Message.Reply(ctx, req)
	if err != nil {
		return fmt.Errorf("lark reply: api request: %w", err)
	}
	if resp == nil || !resp.Success() {
		code, msg := 0, "unknown error"
		if resp != nil {
			code = resp.Code
			msg = resp.Msg
		}
		return fmt.Errorf("lark reply: api code=%d msg=%s", code, msg)
	}
	logger.Info("larkbot: reply sent", "message_id", messageID, "msg_type", msgType)
	return nil
}

// sendMessageWithContent posts a new IM message (file/image) via CreateMessage API.
// Text replies continue to use replyWithContent on the user's message_id.
func (c *Client) sendMessageWithContent(ctx context.Context, cfg *BotConfig, target MessageTarget, msgType, content string) error {
	receiveID, receiveIDType := target.receiveForSend()
	if receiveID == "" || cfg == nil || cfg.AppID == "" || cfg.AppSecret == "" {
		return fmt.Errorf("lark send: missing receive_id or bot credentials")
	}
	body := larkim.NewCreateMessageReqBodyBuilder().
		ReceiveId(receiveID).
		MsgType(msgType).
		Content(content).
		Build()
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(body).
		Build()

	resp, err := newLarkAPIClient(cfg).Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("lark send: api request: %w", err)
	}
	if resp == nil || !resp.Success() {
		code, msg := 0, "unknown error"
		if resp != nil {
			code = resp.Code
			msg = resp.Msg
		}
		return fmt.Errorf("lark send: api code=%d msg=%s", code, msg)
	}
	logger.Info("larkbot: attachment message sent",
		"msg_type", msgType,
		"receive_id_type", receiveIDType,
		"chat_id", target.ChatID,
		"open_id", target.OpenID,
	)
	return nil
}

// replyTextMessage kept for callers/tests; routes through replyMessage.
func (c *Client) replyTextMessage(ctx context.Context, cfg *BotConfig, messageID, text string) error {
	return c.replyMessage(ctx, cfg, messageID, text)
}
