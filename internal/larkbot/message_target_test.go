package larkbot

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestMessageTarget_receiveForSend(t *testing.T) {
	t.Run("prefers chat_id", func(t *testing.T) {
		target := MessageTarget{ChatID: "oc_abc", OpenID: "ou_xyz"}
		id, typ := target.receiveForSend()
		if id != "oc_abc" || typ != larkim.CreateMessageV1ReceiveIDTypeChatId {
			t.Fatalf("got id=%q type=%q", id, typ)
		}
	})
	t.Run("falls back to open_id", func(t *testing.T) {
		target := MessageTarget{OpenID: "ou_xyz"}
		id, typ := target.receiveForSend()
		if id != "ou_xyz" || typ != larkim.CreateMessageV1ReceiveIDTypeOpenId {
			t.Fatalf("got id=%q type=%q", id, typ)
		}
	})
	t.Run("empty when no ids", func(t *testing.T) {
		target := MessageTarget{}
		id, typ := target.receiveForSend()
		if id != "" || typ != "" {
			t.Fatalf("got id=%q type=%q", id, typ)
		}
	})
}

func TestMessageTarget_canReplyText(t *testing.T) {
	empty := MessageTarget{}
	if empty.canReplyText() {
		t.Fatal("empty target should not reply")
	}
	withReply := MessageTarget{ReplyMessageID: "om_1"}
	if !withReply.canReplyText() {
		t.Fatal("should reply when message id set")
	}
}

func TestMessageTarget_canSendAttachment(t *testing.T) {
	empty := MessageTarget{}
	if empty.canSendAttachment() {
		t.Fatal("empty target should not send")
	}
	withChat := MessageTarget{ChatID: "oc_1"}
	if !withChat.canSendAttachment() {
		t.Fatal("should send with chat_id")
	}
}
