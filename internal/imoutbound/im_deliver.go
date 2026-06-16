package imoutbound

import (
	"context"
	"strings"

	"github.com/fisk086/aiops/internal/logger"
)

// DeliverInput is one IM bot turn after the agent has replied.
type DeliverInput struct {
	Channel     string
	Ctx         context.Context // bot scope ctx (for salvage register)
	AgentCtx    context.Context // ctx the agent used (for staged files)
	Scope       Scope
	Store       *Store
	UserRequest string
	AgentText   string
}

// DeliverOutput is sanitized text plus files to upload separately.
type DeliverOutput struct {
	Text        string
	FileNames   []string
	MarkerFiles []string
}

// DeliverIMReply resolves staged attachments, salvages pasted content, and sanitizes reply text.
func DeliverIMReply(in DeliverInput) DeliverOutput {
	store := in.Store
	if store == nil {
		store = GlobalStore()
	}
	agentCtx := in.AgentCtx
	if agentCtx == nil {
		agentCtx = in.Ctx
	}

	replyText, markers := ParseFileMarkers(in.AgentText)
	fileNames := AttachmentNamesForSend(agentCtx, markers)

	salvaged := false
	if len(fileNames) == 0 && ContainsSalvageableIMPayload(in.AgentText) {
		if name, ok := SalvageIMInlinePayload(in.Ctx, store, in.Scope, in.AgentText, in.UserRequest); ok {
			fileNames = AttachmentNamesForSend(in.Ctx, markers)
			if len(fileNames) == 0 {
				fileNames = []string{name}
			}
			salvaged = true
		}
	}

	var text string
	if len(fileNames) > 0 && (salvaged || ContainsSalvageableIMPayload(in.AgentText)) {
		text = ShortIMAttachmentReply(fileNames[0])
	} else {
		text = SanitizeIMReplyText(replyText, in.Scope, store, fileNames)
	}

	if len(fileNames) == 0 && LooksLikeImageGenerationRequest(in.UserRequest) {
		text = ImageGenerationUnavailableUserText(in.Channel)
	} else if len(fileNames) == 0 && NeedsFileRetry(in.UserRequest, in.AgentText, markers, fileNames) {
		text = appendFileDeliveryFailureNotice(text)
	}

	LogAttachmentPipeline(in.Channel, in.Scope, markers, fileNames, agentCtx)
	warnDeliveryAnomalies(in.Channel, in.Scope, markers, fileNames, in.AgentText, text)

	return DeliverOutput{Text: text, FileNames: fileNames, MarkerFiles: markers}
}

// NeedsFileRetry reports whether the bot should ask the agent to invoke IM file tools again.
func NeedsFileRetry(userRequest, agentText string, markers, fileNames []string) bool {
	return len(fileNames) == 0 && (len(markers) > 0 || LooksLikeFileDeliveryRequest(userRequest) || ContainsSalvageableIMPayload(agentText) || strings.Contains(agentText, "http://") || strings.Contains(agentText, "https://") || strings.Contains(agentText, "/api/v1/chat/"))
}

func appendFileDeliveryFailureNotice(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "抱歉，附件未能生成。请再试一次，或说明需要的文件名和内容。"
	}
	return text + "\n\n（附件未能发送，请重新描述需要的文件。）"
}

func warnDeliveryAnomalies(channel string, scope Scope, markers, fileNames []string, agentText, replyText string) {
	if strings.Contains(replyText, "http://") || strings.Contains(replyText, "https://") || strings.Contains(replyText, "/api/v1/chat/") {
		logger.Warn(channel+": reply still contains URL after IM sanitize",
			"session_id", scope.SessionID, "reply_preview", truncatePreview(replyText, 200))
	}
	if len(markers) > len(fileNames) {
		logger.Warn(channel+": attachment markers without staged files", "markers", markers, "staged", fileNames)
	}
	if len(fileNames) == 0 && (strings.Contains(agentText, "/api/v1/chat/") || strings.Contains(agentText, "http://") || strings.Contains(agentText, "https://")) {
		logger.Error(channel+": agent returned URLs but no IM attachments staged",
			"session_id", scope.SessionID, "agent_id", scope.AgentID,
			"hint", "check imoutbound: attachment pipeline reason in logs")
	}
}

func truncatePreview(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
