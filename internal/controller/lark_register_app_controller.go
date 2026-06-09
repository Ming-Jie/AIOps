package controller

import (
	"context"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/fisk086/aiops/internal/larkbot"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
)

type LarkRegisterAppStartRequest struct {
	AutoBind *bool  `json:"auto_bind"`
	TenantBrand string `json:"tenant_brand,omitempty"`
	AppName     string `json:"app_name,omitempty"`
}

func appendAllowedOpenID(cfg *schema.IMConfig, openID string) {
	openID = strings.TrimSpace(openID)
	if openID == "" || cfg == nil {
		return
	}
	for _, existing := range cfg.AllowedUsers {
		if strings.TrimSpace(existing) == openID {
			return
		}
	}
	cfg.AllowedUsers = append(cfg.AllowedUsers, openID)
}

func (c *AgentController) bindLarkCredentials(agentID int64, appID, appSecret, tenantBrand, operatorOpenID string) (bool, error) {
	full, err := c.agentService.GetAgent(agentID)
	if err != nil || full == nil {
		return false, err
	}
	rp := full.RuntimeProfile
	if rp == nil {
		rp = &schema.RuntimeProfile{}
	}
	cfg := rp.IMConfig
	cfg.AppID = strings.TrimSpace(appID)
	cfg.AppSecret = strings.TrimSpace(appSecret)
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return false, nil
	}
	rp.IMEnabled = "lark"
	if strings.EqualFold(strings.TrimSpace(tenantBrand), "lark") || strings.Contains(strings.ToLower(cfg.LarkOpenDomain), "larksuite") {
		cfg.LarkOpenDomain = "https://open.larksuite.com"
	} else if strings.TrimSpace(cfg.LarkOpenDomain) == "" {
		cfg.LarkOpenDomain = "https://open.feishu.cn"
	}
	appendAllowedOpenID(&cfg, operatorOpenID)
	rp.IMConfig = cfg

	_, err = c.agentService.UpdateAgent(agentID, &schema.UpdateAgentRequest{
		RuntimeProfile: rp,
	})
	if err != nil {
		return false, err
	}
	updated, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return false, err
	}
	if updated != nil {
		c.runtime.RegisterAgent(updated)
		c.registerLarkBotIfNeeded(updated)
	}
	return true, nil
}

// StartLarkRegisterAppSession POST /api/v1/agents/:id/im/lark/register-app
func (c *AgentController) StartLarkRegisterAppSession(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "id")
	if agentID < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	agent, err := c.agentService.GetAgent(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}

	var req LarkRegisterAppStartRequest
	if len(hc.Request.Body()) > 0 {
		if err := hc.BindJSON(&req); err != nil {
			hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid JSON body"))
			return
		}
	}
	autoBind := true
	if req.AutoBind != nil {
		autoBind = *req.AutoBind
	}

	mgr := larkbot.GlobalRegisterSessions()
	opts := larkbot.RegisterAppStartOptions{
		AgentID:        agentID,
		AgentName:      agent.Name,
		LarkOpenDomain: "",
		AppPresetName:  strings.TrimSpace(req.AppName),
		Source:         "aiops",
	}
	if agent.RuntimeProfile != nil {
		opts.LarkOpenDomain = agent.RuntimeProfile.IMConfig.LarkOpenDomain
	}
	if autoBind {
		opts.OnCompleted = func(appID, appSecret, tenantBrand, operatorOpenID string) (bool, error) {
			return c.bindLarkCredentials(agentID, appID, appSecret, tenantBrand, operatorOpenID)
		}
	}

	sessionID, err := mgr.Start(ctx, opts)
	if err != nil {
		logger.Warn("lark register-app start failed", "agent_id", agentID, "err", err)
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]string{"session_id": sessionID}))
}

// GetLarkRegisterAppSession GET /api/v1/agents/:id/im/lark/register-app/:sessionId
func (c *AgentController) GetLarkRegisterAppSession(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "id")
	if agentID < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	sessionID := hc.Param("sessionId")
	if sessionID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id required"))
		return
	}
	sess, ok := larkbot.GlobalRegisterSessions().Get(sessionID)
	if !ok || sess.SessionID == "" {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("session not found"))
		return
	}
	_ = agentID
	hc.JSON(http.StatusOK, schema.SuccessResponse(sess))
}

// CancelLarkRegisterAppSession DELETE /api/v1/agents/:id/im/lark/register-app/:sessionId
func (c *AgentController) CancelLarkRegisterAppSession(ctx context.Context, hc *app.RequestContext) {
	sessionID := hc.Param("sessionId")
	if sessionID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id required"))
		return
	}
	if !larkbot.GlobalRegisterSessions().Cancel(sessionID) {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("session not found"))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]bool{"cancelled": true}))
}
