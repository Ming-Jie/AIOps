package larkbot

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/aiops/internal/logger"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func getUserIDFromReactionUser (userID *larkim.UserId) string {
	if userID == nil {
		return ""
	}
	if userID.OpenId != nil && *userID.OpenId != "" {
		return *userID.OpenId
	}
	if userID.UnionId != nil && *userID.UnionId != "" {
		return *userID.UnionId
	}
	if userID.UserId != nil && *userID.UserId != "" {
		return *userID.UserId
	}
	return ""
}

func getReactionEmojiType (emoji *larkim.Emoji) string {
	if emoji == nil || emoji.EmojiType == nil {
		return ""
	}
	return strings.TrimSpace(*emoji.EmojiType)
}

func getAppIDFromReactionEvent (event *larkim.P2MessageReactionCreatedV1) string {
	if event != nil && event.EventV2Base != nil && event.EventV2Base.Header != nil {
		return event.EventV2Base.Header.AppID
	}
	return ""
}

func getAppIDFromReactionDeletedEvent (event *larkim.P2MessageReactionDeletedV1) string {
	if event != nil && event.EventV2Base != nil && event.EventV2Base.Header != nil {
		return event.EventV2Base.Header.AppID
	}
	return ""
}

// handleReactionCreated handles im.message.reaction.created_v1 (user emoji on a message).
// Bot self-reactions (ack "Get") are ignored to avoid noise and redelivery issues.
func (c *Client) handleReactionCreated (_ context.Context, event *larkim.P2MessageReactionCreatedV1) error {
	if event == nil || event.Event == nil {
		return nil
	}
	ev := event.Event
	if getStringPtr(ev.OperatorType) == "app" {
		return nil
	}
	appID := getAppIDFromReactionEvent(event)
	senderID := getUserIDFromReactionUser(ev.UserId)
	emojiType := getReactionEmojiType(ev.ReactionType)
	messageID := getStringPtr(ev.MessageId)

	cfg := c.GetBotConfig(appID)
	if cfg != nil && !c.isSenderAllowed(senderID, cfg.AllowedUsers) {
		return nil
	}

	logger.Info("larkbot: user reaction received",
		"message_id", messageID,
		"emoji_type", emojiType,
		"sender_id", senderID,
		"app_id", appID,
	)
	return nil
}

// handleReactionDeleted handles im.message.reaction.deleted_v1.
func (c *Client) handleReactionDeleted (_ context.Context, event *larkim.P2MessageReactionDeletedV1) error {
	if event == nil || event.Event == nil {
		return nil
	}
	ev := event.Event
	if getStringPtr(ev.OperatorType) == "app" {
		return nil
	}
	appID := getAppIDFromReactionDeletedEvent(event)
	senderID := getUserIDFromReactionUser(ev.UserId)
	emojiType := getReactionEmojiType(ev.ReactionType)
	messageID := getStringPtr(ev.MessageId)

	cfg := c.GetBotConfig(appID)
	if cfg != nil && !c.isSenderAllowed(senderID, cfg.AllowedUsers) {
		return nil
	}

	logger.Info("larkbot: user reaction removed",
		"message_id", messageID,
		"emoji_type", emojiType,
		"sender_id", senderID,
		"app_id", appID,
	)
	return nil
}

func (c *Client) addAckReaction(ctx context.Context, cfg *BotConfig, messageID string) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" || cfg == nil {
		return
	}
	emojiType := DefaultAckEmojiType
	if err := c.addMessageReaction(ctx, cfg, messageID, emojiType); err != nil {
		logger.Warn("larkbot: ack reaction failed", "message_id", messageID, "emoji_type", emojiType, "err", err)
	}
}

func (c *Client) addMessageReaction(ctx context.Context, cfg *BotConfig, messageID, emojiType string) error {
	messageID = strings.TrimSpace(messageID)
	emojiType = strings.TrimSpace(emojiType)
	if messageID == "" || emojiType == "" {
		return nil
	}
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return fmt.Errorf("lark reaction: missing bot credentials")
	}

	req := larkim.NewCreateMessageReactionReqBuilder().
		MessageId(messageID).
		Body(larkim.NewCreateMessageReactionReqBodyBuilder().
			ReactionType(larkim.NewEmojiBuilder().EmojiType(emojiType).Build()).
			Build()).
		Build()

	resp, err := newLarkAPIClient(cfg).Im.MessageReaction.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("lark reaction: api request: %w", err)
	}
	if resp == nil || !resp.Success() {
		code, msg := 0, "unknown error"
		if resp != nil {
			code = resp.Code
			msg = resp.Msg
		}
		return fmt.Errorf("lark reaction: api code=%d msg=%s", code, msg)
	}
	logger.Info("larkbot: ack reaction sent", "message_id", messageID, "emoji_type", emojiType)
	return nil
}
