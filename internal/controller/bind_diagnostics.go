package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
)

const bindBodyPreviewMax = 256

// bindChatRequest binds POST /chat JSON with diagnostics for common client mistakes
// (missing Content-Type, wrong field names) that otherwise leave ChatRequest at zero values.
func bindChatRequest(hc *app.RequestContext, req *schema.ChatRequest, route string) error {
	body := hc.Request.Body()
	bodyLen := len(body)
	contentType := string(hc.ContentType())

	if bodyLen > 0 && !isJSONContentType(contentType) {
		hint := fmt.Sprintf(
			"request body (%d bytes) requires Content-Type: application/json (received %q)",
			bodyLen, contentType,
		)
		logger.Warn("chat request body not parsed",
			"route", route,
			"reason", "missing_or_wrong_content_type",
			"content_type", contentType,
			"body_len", bodyLen,
			"body_preview", previewRequestBody(body),
			"hint", hint,
		)
		return errors.New(hint)
	}

	if err := hc.BindJSON(req); err != nil {
		logger.Warn("chat request JSON bind failed",
			"route", route,
			"content_type", contentType,
			"body_len", bodyLen,
			"body_preview", previewRequestBody(body),
			"err", err,
		)
		return fmt.Errorf("invalid JSON body")
	}

	if hint := chatRequestEmptyAfterBindHint(req, bodyLen); hint != "" {
		logger.Warn("chat request bound with empty fields",
			"route", route,
			"content_type", contentType,
			"body_len", bodyLen,
			"body_preview", previewRequestBody(body),
			"hint", hint,
		)
		return errors.New(hint)
	}
	return nil
}

func isJSONContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "" {
		return false
	}
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct == "application/json" || ct == "application/json-patch+json"
}

func chatRequestHasTargetOrContent(req *schema.ChatRequest) bool {
	if req == nil {
		return false
	}
	if req.AgentID >= 1 || req.WorkflowID >= 1 {
		return true
	}
	if strings.TrimSpace(req.Message) != "" {
		return true
	}
	if req.ImageBase64 != "" || req.ImageURL != "" || len(req.ImageURLs) > 0 || len(req.ImageParts) > 0 {
		return true
	}
	return len(req.FileURLs) > 0
}

func chatRequestEmptyAfterBindHint(req *schema.ChatRequest, bodyLen int) string {
	if chatRequestHasTargetOrContent(req) {
		return ""
	}
	if bodyLen == 0 {
		return "agent_id (>=1) or workflow_id is required"
	}
	return fmt.Sprintf(
		"request body (%d bytes) parsed but agent_id/workflow_id/message are empty; use snake_case JSON keys (agent_id, workflow_id, message)",
		bodyLen,
	)
}

func previewRequestBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > bindBodyPreviewMax {
		return s[:bindBodyPreviewMax] + "..."
	}
	return s
}
