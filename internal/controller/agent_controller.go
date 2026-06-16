package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
)

type AgentController struct {
	agentService *service.AgentService
	chatService  *service.ChatService
	runtime      *agent.Runtime
	rbacService  *service.RBACService
	jwtCfg       auth.JWTConfig
	userStore    storage.UserStore
	imManager    IMManagerInterface
}

type IMManagerInterface interface {
	RegisterAgent(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error
	UnregisterAgent(agentID int64)
}

func NewAgentController(agentService *service.AgentService, chatService *service.ChatService, runtime *agent.Runtime, jwtCfg auth.JWTConfig, userStore storage.UserStore, rbacService ...*service.RBACService) *AgentController {
	ctrl := &AgentController{
		agentService: agentService,
		chatService:  chatService,
		runtime:      runtime,
		jwtCfg:       jwtCfg,
		userStore:    userStore,
	}
	if len(rbacService) > 0 {
		ctrl.rbacService = rbacService[0]
	}
	globalAgentController = ctrl
	return ctrl
}

func (c *AgentController) SetIMManager(mgr IMManagerInterface) {
	c.imManager = mgr
}

func (c *AgentController) registerLarkBotIfNeeded(agent *schema.AgentWithRuntime) {
	c.registerIMBotIfNeeded(agent)
}

func (c *AgentController) registerIMBotIfNeeded(agent *schema.AgentWithRuntime) {
	if c.imManager == nil || agent == nil || agent.RuntimeProfile == nil {
		return
	}

	if err := c.imManager.RegisterAgent(agent, c.runtime); err != nil {
		logger.Error("failed to register im bot for agent", "agent_id", agent.ID, "err", err)
	}
}

func (c *AgentController) unregisterLarkBotIfNeeded(agentID int64) {
	c.unregisterIMBotIfNeeded(agentID)
}

func (c *AgentController) unregisterIMBotIfNeeded(agentID int64) {
	if c.imManager == nil {
		return
	}
	c.imManager.UnregisterAgent(agentID)
}

func (c *AgentController) isChatUser(chatUserIDs []int64, userID int64) bool {
	for _, id := range chatUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func (c *AgentController) GetAgentByID(id int64) (*schema.AgentWithRuntime, error) {
	return c.agentService.GetAgent(id)
}

func (c *AgentController) RegisterLarkBotForAgent(agent *schema.AgentWithRuntime) {
	c.registerLarkBotIfNeeded(agent)
}

// SetLarkIMWsEnabled persists im_config.ws_enabled and refreshes in-memory Lark bot registration.
func (c *AgentController) SetLarkIMWsEnabled(agentID int64, enabled bool) error {
	full, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if full == nil || full.RuntimeProfile == nil {
		return errors.New("agent runtime profile missing")
	}
	if full.RuntimeProfile.IMEnabled != "lark" {
		return errors.New("agent im is not lark")
	}
	v := enabled
	full.RuntimeProfile.IMConfig.WsEnabled = &v
	if _, err := c.agentService.UpdateAgent(agentID, &schema.UpdateAgentRequest{
		RuntimeProfile: full.RuntimeProfile,
	}); err != nil {
		return err
	}
	updated, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if updated != nil {
		c.registerLarkBotIfNeeded(updated)
	}
	return nil
}

func (c *AgentController) UnregisterLarkBotForAgent(agentID int64) {
	c.unregisterLarkBotIfNeeded(agentID)
}

func (c *AgentController) RegisterTelegramBotForAgent(agent *schema.AgentWithRuntime) {
	c.registerIMBotIfNeeded(agent)
}

// SetTelegramIMWsEnabled persists im_config.ws_enabled and refreshes in-memory Telegram bot registration.
func (c *AgentController) SetTelegramIMWsEnabled(agentID int64, enabled bool) error {
	full, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if full == nil || full.RuntimeProfile == nil {
		return errors.New("agent runtime profile missing")
	}
	if full.RuntimeProfile.IMEnabled != "telegram" {
		return errors.New("agent im is not telegram")
	}
	v := enabled
	full.RuntimeProfile.IMConfig.WsEnabled = &v
	if _, err := c.agentService.UpdateAgent(agentID, &schema.UpdateAgentRequest{
		RuntimeProfile: full.RuntimeProfile,
	}); err != nil {
		return err
	}
	updated, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if updated != nil {
		c.registerIMBotIfNeeded(updated)
	}
	return nil
}

func (c *AgentController) UnregisterTelegramBotForAgent(agentID int64) {
	c.unregisterIMBotIfNeeded(agentID)
}

func (c *AgentController) RegisterDingtalkBotForAgent(agent *schema.AgentWithRuntime) {
	c.registerIMBotIfNeeded(agent)
}

// SetDingtalkIMWsEnabled persists im_config.ws_enabled and refreshes in-memory DingTalk bot registration.
func (c *AgentController) SetDingtalkIMWsEnabled(agentID int64, enabled bool) error {
	full, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if full == nil || full.RuntimeProfile == nil {
		return errors.New("agent runtime profile missing")
	}
	if full.RuntimeProfile.IMEnabled != "dingtalk" {
		return errors.New("agent im is not dingtalk")
	}
	v := enabled
	full.RuntimeProfile.IMConfig.WsEnabled = &v
	if _, err := c.agentService.UpdateAgent(agentID, &schema.UpdateAgentRequest{
		RuntimeProfile: full.RuntimeProfile,
	}); err != nil {
		return err
	}
	updated, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if updated != nil {
		c.registerIMBotIfNeeded(updated)
	}
	return nil
}

func (c *AgentController) UnregisterDingtalkBotForAgent(agentID int64) {
	c.unregisterIMBotIfNeeded(agentID)
}

func (c *AgentController) RegisterQQBotForAgent(agent *schema.AgentWithRuntime) {
	c.registerIMBotIfNeeded(agent)
}

func (c *AgentController) SetQQIMWsEnabled(agentID int64, enabled bool) error {
	full, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if full == nil || full.RuntimeProfile == nil {
		return errors.New("agent runtime profile missing")
	}
	if full.RuntimeProfile.IMEnabled != "qq" {
		return errors.New("agent im is not qq")
	}
	v := enabled
	full.RuntimeProfile.IMConfig.WsEnabled = &v
	if _, err := c.agentService.UpdateAgent(agentID, &schema.UpdateAgentRequest{
		RuntimeProfile: full.RuntimeProfile,
	}); err != nil {
		return err
	}
	updated, err := c.agentService.GetAgent(agentID)
	if err != nil {
		return err
	}
	if updated != nil {
		c.registerIMBotIfNeeded(updated)
	}
	return nil
}

func (c *AgentController) UnregisterQQBotForAgent(agentID int64) {
	c.unregisterIMBotIfNeeded(agentID)
}

func (c *AgentController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (c *AgentController) RegisterRoutes(r *server.Hertz) {
	agents := r.Group("/api/v1/agents")
	if c.userStore != nil {
		agents.Use(auth.OptionalJWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	}
	agents.GET("", c.ListAgents)
	agents.GET("/all", c.ListAllAgents)
	agents.GET("/for-schedule", c.ListAgentsForSchedule)
	agents.GET("/:id", c.GetAgent)
	agents.POST("", c.CreateAgent)
	agents.PUT("/:id", c.UpdateAgent)
	agents.DELETE("/:id", c.DeleteAgent)
	agents.GET("/:id/capability-tree", c.GetCapabilityTree)
	agents.PUT("/:id/capability-tree", c.UpdateCapabilityTree)
	agents.GET("/:id/capabilities", c.GetCapabilities)
	agents.POST("/:id/im/lark/register-app", c.StartLarkRegisterAppSession)
	agents.GET("/:id/im/lark/register-app/:sessionId", c.GetLarkRegisterAppSession)
	agents.DELETE("/:id/im/lark/register-app/:sessionId", c.CancelLarkRegisterAppSession)
}

// @Summary List all agents
// @Description Get a list of all agents
// @Tags agents
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents [get]
func (c *AgentController) ListAgents(ctx context.Context, hc *app.RequestContext) {
	agents, err := c.agentService.ListAgents()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	user := auth.GetCurrentUser(hc)
	result := make([]schema.Agent, len(agents))
	for i, a := range agents {
		ag := *a
		if user != nil {
			ag.CanEdit = user.IsAdmin || ag.CreatedBy == user.ID
			ag.CanChat = ag.CanEdit || c.isChatUser(ag.ChatUserIDs, user.ID)
		}
		result[i] = ag
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(result))
}

// @Summary List all agents (admin only, no RBAC filter)
// @Description Get a list of all agents without RBAC filtering
// @Tags agents
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/all [get]
func (c *AgentController) ListAllAgents(ctx context.Context, hc *app.RequestContext) {
	agents, err := c.agentService.ListAgents()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(agents))
}

// @Summary List agents available for schedule
// @Description Get a list of agents filtered by excluding those with client execution mode
// @Tags agents
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/for-schedule [get]
func (c *AgentController) ListAgentsForSchedule(ctx context.Context, hc *app.RequestContext) {
	agents, err := c.agentService.ListAgentsForSchedule()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(agents))
}

// @Summary Get agent by ID
// @Description Get a single agent by its ID
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id} [get]
func (c *AgentController) GetAgent(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	agent, err := c.agentService.GetAgent(id)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}

	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}

	// 只有创建者、可对话用户、管理员能看到完整 RuntimeProfile（含 LLM 配置、IM token 等敏感信息）
	canViewDetail := user.IsAdmin || agent.CreatedBy == user.ID || c.isChatUser(agent.ChatUserIDs, user.ID)

	if !canViewDetail {
		// 其他人只看基本信息，隐藏运行时敏感配置
		agent.RuntimeProfile = nil
	}

	agent.CanEdit = user.IsAdmin || agent.CreatedBy == user.ID
	agent.CanChat = agent.CanEdit || c.isChatUser(agent.ChatUserIDs, user.ID)

	hc.JSON(http.StatusOK, schema.SuccessResponse(agent))
}

// @Summary Create a new agent
// @Description Create a new agent with the specified configuration
// @Tags agents
// @Accept json
// @Produce json
// @Param agent body schema.CreateAgentRequest true "Agent data"
// @Success 201 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents [post]
func (c *AgentController) CreateAgent(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateAgentRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	createdBy := int64(0)
	if user := auth.GetCurrentUser(hc); user != nil {
		createdBy = user.ID
	}

	agent, err := c.agentService.CreateAgent(&req, createdBy)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	fullAgent, _ := c.agentService.GetAgent(agent.ID)
	if fullAgent != nil {
		c.runtime.RegisterAgent(fullAgent)
		c.registerLarkBotIfNeeded(fullAgent)
	}

	// 补齐 chat_user_ids 返回
	agent.ChatUserIDs = req.ChatUserIDs

	hc.JSON(http.StatusCreated, schema.SuccessResponse(agent))
}

// @Summary Update an agent
// @Description Update an existing agent by its ID
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Param agent body schema.UpdateAgentRequest true "Agent data"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id} [put]
func (c *AgentController) checkAgentOwner(ctx context.Context, hc *app.RequestContext, agentID int64) bool {
	user := auth.GetCurrentUser(hc)
	if user == nil || user.IsAdmin {
		return true
	}
	ownerID, err := c.agentService.GetAgentOwner(agentID)
	if err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return false
	}
	if ownerID != user.ID {
		hc.JSON(http.StatusForbidden, schema.ErrorResponse("you do not have permission to modify this agent"))
		return false
	}
	return true
}

func (c *AgentController) UpdateAgent(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	if !c.checkAgentOwner(ctx, hc, id) {
		return
	}

	var req schema.UpdateAgentRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	oldAgent, _ := c.agentService.GetAgent(id)

	agent, err := c.agentService.UpdateAgent(id, &req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}

	fullAgent, _ := c.agentService.GetAgent(id)
	if fullAgent != nil {
		c.runtime.RegisterAgent(fullAgent)
		if imConfigChanged(oldAgent, fullAgent) {
			c.registerLarkBotIfNeeded(fullAgent)
		}
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(agent))
}

// @Summary Delete an agent
// @Description Delete an agent by its ID
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id} [delete]
func (c *AgentController) DeleteAgent(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	if !c.checkAgentOwner(ctx, hc, id) {
		return
	}

	if err := c.agentService.DeleteAgent(id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	c.runtime.UnregisterAgent(id)
	c.unregisterLarkBotIfNeeded(id)

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

// @Summary Get agent capability tree
// @Description Get the capability tree structure for an agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id}/capability-tree [get]
func (c *AgentController) GetCapabilityTree(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	tree, err := c.agentService.GetCapabilityTree(id)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(tree))
}

// @Summary Update agent capability tree
// @Description Update the capability tree structure for an agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Param tree body schema.UpdateCapabilityTreeRequest true "Capability tree data"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id}/capability-tree [put]
func (c *AgentController) UpdateCapabilityTree(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	var req schema.UpdateCapabilityTreeRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	tree, err := c.agentService.UpdateCapabilityTree(id, req.Nodes)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	fullAgent, _ := c.agentService.GetAgent(id)
	if fullAgent != nil {
		fullAgent.CapabilityTree = tree
		c.runtime.RegisterAgent(fullAgent)
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(tree))
}

// @Summary Get agent capabilities
// @Description Get the resolved capabilities for an agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Router /agents/{id}/capabilities [get]
func (c *AgentController) GetCapabilities(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	capabilities := c.chatService.GetCapabilities(id)
	hc.JSON(http.StatusOK, schema.SuccessResponse(capabilities))
}

// imConfigChanged returns true when the IM-related configuration differs between old and new agent.
// Only when IM fields change should we trigger IM bot re-registration on agent update.
func imConfigChanged(old, new *schema.AgentWithRuntime) bool {
	if old == nil && new == nil {
		return false
	}
	if old == nil || new == nil {
		return true
	}
	var (
		oldProfile, newProfile *schema.RuntimeProfile
	)
	if old.RuntimeProfile != nil {
		oldProfile = old.RuntimeProfile
	}
	if new.RuntimeProfile != nil {
		newProfile = new.RuntimeProfile
	}
	if (oldProfile == nil) != (newProfile == nil) {
		return true
	}
	if oldProfile == nil {
		return false
	}
	if oldProfile.IMEnabled != newProfile.IMEnabled {
		return true
	}
	return !imConfigEqual(oldProfile.IMConfig, newProfile.IMConfig)
}

// imConfigEqual compares two IMConfig structs, normalising nil/empty slices and nil/false pointers
// to avoid false positives caused by JSON round-trip differences.
func imConfigEqual(a, b schema.IMConfig) bool {
	if a.AppID != b.AppID {
		return false
	}
	if a.AppSecret != b.AppSecret {
		return false
	}
	if a.TelegramToken != b.TelegramToken {
		return false
	}
	if a.TelegramChatID != b.TelegramChatID {
		return false
	}
	if a.WebhookURL != b.WebhookURL {
		return false
	}
	if a.Secret != b.Secret {
		return false
	}
	if a.BotName != b.BotName {
		return false
	}
	if a.AutoReply != b.AutoReply {
		return false
	}
	if a.NotifyOnApproval != b.NotifyOnApproval {
		return false
	}
	if a.LarkRegion != b.LarkRegion {
		return false
	}
	if a.LarkOpenDomain != b.LarkOpenDomain {
		return false
	}
	if a.QQAppID != b.QQAppID {
		return false
	}
	if a.QQBotToken != b.QQBotToken {
		return false
	}
	if !ptrBoolEqual(a.WsEnabled, b.WsEnabled) {
		return false
	}
	return strSliceEqual(a.AllowedUsers, b.AllowedUsers)
}

func ptrBoolEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil {
		return !*b
	}
	if b == nil {
		return !*a
	}
	return *a == *b
}

func strSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var globalAgentController *AgentController

func GetAgentController() *AgentController {
	return globalAgentController
}

func init() {
	logger.Debug("agent controller package initialized")
}
