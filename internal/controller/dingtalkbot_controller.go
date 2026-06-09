package controller

import (
	"context"
	"net/http"
	"sort"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/aiops/internal/dingtalkbot"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
)

type DingtalkBotStatus struct {
	AppID     string `json:"app_id"`
	AgentID   int64  `json:"agent_id"`
	AgentName string `json:"agent_name"`
	IsRunning bool   `json:"is_running"`
}

func NewDingtalkBotController(agentService *service.AgentService) *DingtalkBotController {
	return &DingtalkBotController{agentService: agentService}
}

type DingtalkBotController struct {
	agentService *service.AgentService
}

func (c *DingtalkBotController) RegisterRoutes(r *server.Hertz) {
	r.GET("/api/v1/dingtalkbots", c.ListBots)
	r.POST("/api/v1/dingtalkbots/start", c.Start)
	r.POST("/api/v1/dingtalkbots/stop", c.Stop)
	r.POST("/api/v1/dingtalkbots/:agentId/register", c.RegisterForAgent)
	r.POST("/api/v1/dingtalkbots/:agentId/ws/start", c.StartAgentWS)
	r.POST("/api/v1/dingtalkbots/:agentId/ws/stop", c.StopAgentWS)
	r.DELETE("/api/v1/dingtalkbots/:agentId", c.UnregisterForAgent)
}

func (c *DingtalkBotController) ListBots(ctx context.Context, hc *app.RequestContext) {
	client := dingtalkbot.Global()

	var entries []DingtalkBotStatus
	if c.agentService != nil {
		agents, err := c.agentService.ListAgents()
		if err != nil {
			hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
			return
		}
		for _, a := range agents {
			if a == nil {
				continue
			}
			full, err := c.agentService.GetAgent(a.ID)
			if err != nil || full == nil || full.RuntimeProfile == nil {
				continue
			}
			if full.RuntimeProfile.IMEnabled != "dingtalk" {
				continue
			}
			appID := full.RuntimeProfile.IMConfig.AppID
			if appID == "" {
				continue
			}
			inMem := client.GetBotConfig(appID) != nil
			isRunning := inMem && client.IsBotRunning(appID)
			entries = append(entries, DingtalkBotStatus{
				AppID:     appID,
				AgentID:   a.ID,
				AgentName: full.Name,
				IsRunning: isRunning,
			})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].AgentID < entries[j].AgentID })
	} else {
		bots := client.GetBots()
		for appID, entry := range bots {
			entries = append(entries, DingtalkBotStatus{
				AppID:     appID,
				AgentID:   entry.Config.AgentID,
				AgentName: "",
				IsRunning: client.IsBotRunning(appID),
			})
		}
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"bots":      entries,
		"running":   client.IsRunning(),
		"bot_count": len(entries),
	}))
}

func (c *DingtalkBotController) persistAllDingtalkWsEnabled(enabled bool) {
	ctrl := GetAgentController()
	if ctrl == nil || c.agentService == nil {
		return
	}
	agents, err := c.agentService.ListAgents()
	if err != nil {
		logger.Warn("dingtalkbot: list agents for ws_enabled persist failed", "err", err)
		return
	}
	for _, a := range agents {
		if a == nil {
			continue
		}
		full, err := c.agentService.GetAgent(a.ID)
		if err != nil || full == nil || full.RuntimeProfile == nil {
			continue
		}
		if full.RuntimeProfile.IMEnabled != "dingtalk" || full.RuntimeProfile.IMConfig.AppID == "" {
			continue
		}
		if err := ctrl.SetDingtalkIMWsEnabled(a.ID, enabled); err != nil {
			logger.Warn("dingtalkbot: persist ws_enabled failed", "agent_id", a.ID, "enabled", enabled, "err", err)
		}
	}
}

func (c *DingtalkBotController) Start(ctx context.Context, hc *app.RequestContext) {
	client := dingtalkbot.Global()
	if client.GetBotCount() == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("no bots registered"))
		return
	}
	c.persistAllDingtalkWsEnabled(true)
	go client.Start(ctx)
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"started":   true,
		"bot_count": client.GetBotCount(),
	}))
}

func (c *DingtalkBotController) Stop(ctx context.Context, hc *app.RequestContext) {
	client := dingtalkbot.Global()
	client.Stop()
	c.persistAllDingtalkWsEnabled(false)
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"stopped": true,
	}))
}

func (c *DingtalkBotController) RegisterForAgent(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}
	agent, err := ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}
	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "dingtalk" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have dingtalk im enabled"))
		return
	}
	ctrl.RegisterDingtalkBotForAgent(agent)
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"registered": true,
		"agent_id":   agentID,
		"app_id":     agent.RuntimeProfile.IMConfig.AppID,
	}))
}

func (c *DingtalkBotController) UnregisterForAgent(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}
	ctrl.UnregisterDingtalkBotForAgent(agentID)
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"unregistered": true,
		"agent_id":     agentID,
	}))
}

func (c *DingtalkBotController) StartAgentWS(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}
	agent, err := ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}
	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "dingtalk" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have dingtalk im enabled"))
		return
	}
	appID := agent.RuntimeProfile.IMConfig.AppID
	if appID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("dingtalk app_id is empty"))
		return
	}
	if err := ctrl.SetDingtalkIMWsEnabled(agentID, true); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	agent, err = ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}
	ctrl.RegisterDingtalkBotForAgent(agent)
	client := dingtalkbot.Global()
	if client.GetBotConfig(appID) == nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("bot not registered: check App ID/App Secret and save the agent"))
		return
	}
	if err := client.StartBot(context.Background(), appID); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"started": true,
		"app_id":  appID,
	}))
}

func (c *DingtalkBotController) StopAgentWS(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}
	agent, err := ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}
	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "dingtalk" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have dingtalk im enabled"))
		return
	}
	appID := agent.RuntimeProfile.IMConfig.AppID
	if appID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("dingtalk app_id is empty"))
		return
	}
	dingtalkbot.Global().StopBot(appID)
	if err := ctrl.SetDingtalkIMWsEnabled(agentID, false); err != nil {
		logger.Warn("dingtalkbot: persist ws_enabled=false failed", "agent_id", agentID, "err", err)
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"stopped": true,
		"app_id":  appID,
	}))
}
