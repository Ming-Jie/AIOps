package controller

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/larkbot"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
)

type LarkBotStatus struct {
	AppID      string `json:"app_id"`
	AgentID    int64  `json:"agent_id"`
	AgentName  string `json:"agent_name"`
	IsRunning  bool   `json:"is_running"`
}

func NewLarkBotController(agentService *service.AgentService, jwtCfg auth.JWTConfig, userStore storage.UserStore, rbacService ...*service.RBACService) *LarkBotController {
	ctrl := &LarkBotController{agentService: agentService, jwtCfg: jwtCfg, userStore: userStore}
	if len(rbacService) > 0 {
		ctrl.rbacService = rbacService[0]
	}
	return ctrl
}

type LarkBotController struct {
	agentService *service.AgentService
	jwtCfg       auth.JWTConfig
	userStore    storage.UserStore
	rbacService  *service.RBACService
}

func (c *LarkBotController) getUserForMiddleware(userID int64) (*auth.User, error) {
	if c.userStore == nil {
		return nil, errors.New("user store not configured")
	}
	user, err := c.userStore.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Status:   string(user.Status),
		IsAdmin:  user.IsAdmin,
	}, nil
}

func (c *LarkBotController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1")
	if c.userStore != nil {
		g.Use(auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	}
	g.GET("/larkbots", c.ListBots)
	g.POST("/larkbots/start", c.Start)
	g.POST("/larkbots/stop", c.Stop)
	g.POST("/larkbots/refresh", c.Refresh)
	g.POST("/larkbots/:agentId/register", c.RegisterForAgent)
	g.POST("/larkbots/:agentId/ws/start", c.StartAgentWS)
	g.POST("/larkbots/:agentId/ws/stop", c.StopAgentWS)
	g.DELETE("/larkbots/:agentId", c.UnregisterForAgent)
}

func (c *LarkBotController) ListBots(ctx context.Context, hc *app.RequestContext) {
	client := larkbot.Global()

	user := auth.GetCurrentUser(hc)

	var entries []LarkBotStatus
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
			if c.rbacService != nil && user != nil && !user.IsAdmin {
				if !c.rbacService.CheckAgentAccess(ctx, user.ID, a.Name, user.IsAdmin) {
					continue
				}
			}
			full, err := c.agentService.GetAgent(a.ID)
			if err != nil || full == nil || full.RuntimeProfile == nil {
				continue
			}
			if full.RuntimeProfile.IMEnabled != "lark" {
				continue
			}
			appID := full.RuntimeProfile.IMConfig.AppID
			if appID == "" {
				continue
			}
			inMem := client.GetBotConfig(appID) != nil
			isRunning := inMem && client.IsBotWSRunning(appID)
			entries = append(entries, LarkBotStatus{
				AppID:      appID,
				AgentID:    a.ID,
				AgentName:  full.Name,
				IsRunning:  isRunning,
			})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].AgentID < entries[j].AgentID })
	} else {
		bots := client.GetBots()
		for appID, entry := range bots {
			entries = append(entries, LarkBotStatus{
				AppID:      appID,
				AgentID:    entry.Config.AgentID,
				AgentName:  "",
				IsRunning:  client.IsBotWSRunning(appID),
			})
		}
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"bots":      entries,
		"running":   client.IsRunning(),
		"bot_count": len(entries),
	}))
}

func (c *LarkBotController) persistAllLarkWsEnabled(enabled bool) {
	ctrl := GetAgentController()
	if ctrl == nil || c.agentService == nil {
		return
	}
	agents, err := c.agentService.ListAgents()
	if err != nil {
		logger.Warn("larkbot: list agents for ws_enabled persist failed", "err", err)
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
		if full.RuntimeProfile.IMEnabled != "lark" || full.RuntimeProfile.IMConfig.AppID == "" {
			continue
		}
		if err := ctrl.SetLarkIMWsEnabled(a.ID, enabled); err != nil {
			logger.Warn("larkbot: persist ws_enabled failed", "agent_id", a.ID, "enabled", enabled, "err", err)
		}
	}
}

func (c *LarkBotController) Start(ctx context.Context, hc *app.RequestContext) {
	client := larkbot.Global()

	if client.GetBotCount() == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("no bots registered"))
		return
	}

	c.persistAllLarkWsEnabled(true)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("larkbot start panicked", "recover", r)
			}
		}()
		client.Start(ctx)
	}()

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"started":   true,
		"bot_count": client.GetBotCount(),
	}))
}

func (c *LarkBotController) Stop(ctx context.Context, hc *app.RequestContext) {
	client := larkbot.Global()
	client.Stop()
	c.persistAllLarkWsEnabled(false)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"stopped": true,
	}))
}

func (c *LarkBotController) Refresh(ctx context.Context, hc *app.RequestContext) {
	client := larkbot.Global()

	if !client.IsRunning() {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("larkbot not running"))
		return
	}

	client.RefreshAllBots(ctx)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"refreshed": true,
		"bot_count": client.GetBotCount(),
	}))
}

func (c *LarkBotController) RegisterForAgent(ctx context.Context, hc *app.RequestContext) {
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

	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "lark" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have lark im enabled"))
		return
	}

	ctrl.RegisterLarkBotForAgent(agent)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"registered": true,
		"agent_id":   agentID,
		"app_id":     agent.RuntimeProfile.IMConfig.AppID,
	}))
}

func (c *LarkBotController) UnregisterForAgent(ctx context.Context, hc *app.RequestContext) {
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

	ctrl.UnregisterLarkBotForAgent(agentID)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"unregistered": true,
		"agent_id":     agentID,
	}))
}

// StartAgentWS starts the WebSocket for one agent's Lark app (must be registered in memory).
func (c *LarkBotController) StartAgentWS(ctx context.Context, hc *app.RequestContext) {
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
	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "lark" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have lark im enabled"))
		return
	}
	appID := agent.RuntimeProfile.IMConfig.AppID
	if appID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("lark app_id is empty"))
		return
	}
	if err := ctrl.SetLarkIMWsEnabled(agentID, true); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	agent, err = ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}
	client := larkbot.Global()
	if client.GetBotConfig(appID) == nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("bot not registered: check Lark App ID / Secret and save the agent, or ensure IM channel is Lark"))
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

// StopAgentWS stops the WebSocket for one agent without removing registration.
func (c *LarkBotController) StopAgentWS(ctx context.Context, hc *app.RequestContext) {
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
	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "lark" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have lark im enabled"))
		return
	}
	appID := agent.RuntimeProfile.IMConfig.AppID
	if appID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("lark app_id is empty"))
		return
	}
	larkbot.Global().StopBot(appID)
	if err := ctrl.SetLarkIMWsEnabled(agentID, false); err != nil {
		logger.Warn("larkbot: persist ws_enabled=false failed", "agent_id", agentID, "err", err)
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"stopped": true,
		"app_id":  appID,
	}))
}
