package larkbot

import (
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MessageTarget identifies where to reply (text) vs send (file/image) in Lark IM.
type MessageTarget struct {
	ReplyMessageID string // user message to reply to (text)
	ChatID         string // session chat_id for CreateMessage
	OpenID         string // sender open_id fallback for p2p CreateMessage
}

func (t MessageTarget) receiveForSend() (receiveID, receiveIDType string) {
	if id := strings.TrimSpace(t.ChatID); id != "" {
		return id, larkim.CreateMessageV1ReceiveIDTypeChatId
	}
	if id := strings.TrimSpace(t.OpenID); id != "" {
		return id, larkim.CreateMessageV1ReceiveIDTypeOpenId
	}
	return "", ""
}

func (t MessageTarget) canReplyText() bool {
	return strings.TrimSpace(t.ReplyMessageID) != ""
}

func (t MessageTarget) canSendAttachment() bool {
	id, _ := t.receiveForSend()
	return id != ""
}
