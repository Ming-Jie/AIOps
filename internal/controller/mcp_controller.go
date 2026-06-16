package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

var mcpSensitiveKeys = []string{"bearer_token", "api_key", "token"}

type MCPController struct {
	mcpService *service.MCPService
	jwtCfg     auth.JWTConfig
	userStore  storage.UserStore
}

func NewMCPController(mcpService *service.MCPService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *MCPController {
	return &MCPController{mcpService: mcpService, jwtCfg: jwtCfg, userStore: userStore}
}

func (c *MCPController) currentUser(hc *app.RequestContext) *auth.User {
	return auth.GetCurrentUser(hc)
}

func (c *MCPController) checkMCPOwner(hc *app.RequestContext, configID int64) bool {
	user := c.currentUser(hc)
	if user == nil || user.IsAdmin {
		return true
	}
	cfg, err := c.mcpService.GetConfig(configID)
	if err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("config not found"))
		return false
	}
	if cfg == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("config not found"))
		return false
	}
	if cfg.CreatedBy != user.ID {
		hc.JSON(http.StatusForbidden, schema.ErrorResponse("you do not have permission to modify this config"))
		return false
	}
	return true
}

func isMasked(s string) bool {
	return strings.Contains(s, "****")
}

func maskMCPConfig(cfg *schema.MCPConfig) *schema.MCPConfig {
	if cfg == nil || cfg.Config == nil {
		return cfg
	}
	masked := *cfg
	masked.Config = make(map[string]any, len(cfg.Config))
	for k, v := range cfg.Config {
		masked.Config[k] = v
	}
	for _, key := range mcpSensitiveKeys {
		if val, ok := masked.Config[key].(string); ok && val != "" {
			masked.Config[key] = maskKey(val)
		}
	}
	if headers, ok := masked.Config["headers"].(map[string]any); ok {
		maskedHeaders := make(map[string]any, len(headers))
		for k, v := range headers {
			if s, ok := v.(string); ok && s != "" {
				maskedHeaders[k] = maskKey(s)
			} else {
				maskedHeaders[k] = v
			}
		}
		masked.Config["headers"] = maskedHeaders
	}
	return &masked
}

func mergeConfigPreserveSecrets(existing, updated map[string]any) map[string]any {
	if existing == nil {
		return updated
	}
	result := make(map[string]any, len(updated))
	for k, v := range updated {
		result[k] = v
	}
	for _, key := range mcpSensitiveKeys {
		if newVal, ok := result[key].(string); ok && isMasked(newVal) {
			if oldVal, ok := existing[key].(string); ok {
				result[key] = oldVal
			}
		}
	}
	if headers, ok := result["headers"].(map[string]any); ok {
		if oldHeaders, ok := existing["headers"].(map[string]any); ok {
			for k, v := range headers {
				if s, ok := v.(string); ok && isMasked(s) {
					if oldVal, ok := oldHeaders[k].(string); ok {
						headers[k] = oldVal
					}
				}
			}
		}
	}
	return result
}

func (c *MCPController) RegisterRoutes(r *server.Hertz) {
	mcp := r.Group("/api/v1/mcp")
	if c.userStore != nil {
		mcp.Use(auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	}
	mcp.GET("/configs", c.ListConfigs)
	mcp.POST("/configs", c.CreateConfig)
	mcp.GET("/configs/:id/tools", c.ListTools)
	mcp.PUT("/configs/:id", c.UpdateConfig)
	mcp.DELETE("/configs/:id", c.DeleteConfig)
	mcp.POST("/configs/:id/sync", c.SyncServer)
}

// @Summary List all MCP configs
// @Description Get a list of all MCP server configurations
// @Tags mcp
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs [get]
func (c *MCPController) ListConfigs(ctx context.Context, hc *app.RequestContext) {
	configs, err := c.mcpService.ListConfigs()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	user := c.currentUser(hc)
	masked := make([]*schema.MCPConfig, 0, len(configs))
	for _, cfg := range configs {
		if user != nil && !user.IsAdmin && cfg.CreatedBy != 0 && cfg.CreatedBy != user.ID {
			continue
		}
		masked = append(masked, maskMCPConfig(cfg))
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(masked))
}

// @Summary Create a new MCP config
// @Description Create a new MCP server configuration
// @Tags mcp
// @Accept json
// @Produce json
// @Param config body schema.CreateMCPConfigRequest true "MCP config data"
// @Success 201 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs [post]
func (c *MCPController) CreateConfig(ctx context.Context, hc *app.RequestContext) {
	user := c.currentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}

	var req schema.CreateMCPConfigRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	cfg, err := c.mcpService.CreateConfig(&req, user.ID)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusCreated, schema.SuccessResponse(maskMCPConfig(cfg)))
}

// @Summary List tools synced for an MCP config
// @Tags mcp
// @Produce json
// @Param id path int true "MCP Config ID"
// @Success 200 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Router /mcp/configs/{id}/tools [get]
func (c *MCPController) ListTools(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}
	tools, err := c.mcpService.ListTools(id)
	if err != nil {
		if errors.Is(err, storage.ErrMCPConfigNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(tools))
}

// @Summary Update an MCP config
// @Description Update an existing MCP server configuration
// @Tags mcp
// @Accept json
// @Produce json
// @Param id path int true "MCP Config ID"
// @Param config body schema.CreateMCPConfigRequest true "MCP config data"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs/{id} [put]
func (c *MCPController) UpdateConfig(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}

	if !c.checkMCPOwner(hc, id) {
		return
	}

	var req schema.CreateMCPConfigRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	existing, err := c.mcpService.GetConfig(id)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if existing == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("config not found"))
		return
	}
	req.Config = mergeConfigPreserveSecrets(existing.Config, req.Config)

	cfg, err := c.mcpService.UpdateConfig(id, &req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if cfg == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("config not found"))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(maskMCPConfig(cfg)))
}

// @Summary Delete an MCP config
// @Description Delete an MCP server configuration
// @Tags mcp
// @Accept json
// @Produce json
// @Param id path int true "MCP Config ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs/{id} [delete]
func (c *MCPController) DeleteConfig(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}

	if !c.checkMCPOwner(hc, id) {
		return
	}

	if err := c.mcpService.DeleteConfig(id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

// @Summary Sync MCP server tools
// @Description Sync tools from an MCP server
// @Tags mcp
// @Accept json
// @Produce json
// @Param id path int true "MCP Config ID"
// @Param req body schema.SyncMCPServerRequest true "Sync request"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs/{id}/sync [post]
func (c *MCPController) SyncServer(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}

	if !c.checkMCPOwner(hc, id) {
		return
	}

	var req schema.SyncMCPServerRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	if err := c.mcpService.SyncServer(ctx, id, &req); err != nil {
		if errors.Is(err, service.ErrMCPDiscoveryNeedsTarget) {
			hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

func (c *MCPController) getUserForMiddleware(userID int64) (*auth.User, error) {
	user, err := c.userStore.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &auth.User{ID: user.ID, Username: user.Username, Email: user.Email, Status: string(user.Status), IsAdmin: user.IsAdmin}, nil
}
