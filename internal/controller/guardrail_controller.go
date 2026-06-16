package controller

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
)

type GuardrailController struct {
	guardrailService *service.GuardrailService
	rbacService      *service.RBACService
	jwtCfg           auth.JWTConfig
	userStore        storage.UserStore
}

func NewGuardrailController(guardrailService *service.GuardrailService, jwtCfg auth.JWTConfig, userStore storage.UserStore, rbacService ...*service.RBACService) *GuardrailController {
	ctrl := &GuardrailController{
		guardrailService: guardrailService,
		jwtCfg:           jwtCfg,
		userStore:        userStore,
	}
	if len(rbacService) > 0 {
		ctrl.rbacService = rbacService[0]
	}
	return ctrl
}

func (c *GuardrailController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/guardrails")
	if c.userStore != nil {
		g.Use(auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	}

	g.GET("/rules", c.ListRules)
	g.GET("/rules/:id", c.GetRule)
	g.POST("/rules", c.CreateRule)
	g.PUT("/rules/:id", c.UpdateRule)
	g.DELETE("/rules/:id", c.DeleteRule)
	g.POST("/rules/test", c.TestRule)
	g.GET("/logs", c.ListLogs)
}

func (c *GuardrailController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (c *GuardrailController) ListRules(ctx context.Context, hc *app.RequestContext) {
	rules, err := c.guardrailService.ListRules()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(rules))
}

func (c *GuardrailController) GetRule(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid rule id"))
		return
	}
	rule, err := c.guardrailService.GetRule(id)
	if err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(rule))
}

func (c *GuardrailController) CreateRule(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateGuardrailRuleRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	user := auth.GetCurrentUser(hc)
	var userID int64
	if user != nil {
		userID = user.ID
	}
	rule, err := c.guardrailService.CreateRule(&req, userID)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusCreated, schema.SuccessResponse(rule))
}

func (c *GuardrailController) UpdateRule(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid rule id"))
		return
	}

	var req schema.UpdateGuardrailRuleRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	rule, err := c.guardrailService.UpdateRule(id, &req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(rule))
}

func (c *GuardrailController) DeleteRule(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid rule id"))
		return
	}

	if err := c.guardrailService.DeleteRule(id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

func (c *GuardrailController) TestRule(ctx context.Context, hc *app.RequestContext) {
	var req schema.TestGuardrailRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	result, err := c.guardrailService.TestRule(&req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(result))
}

func (c *GuardrailController) ListLogs(ctx context.Context, hc *app.RequestContext) {
	req := schema.ListGuardrailLogsRequest{
		RuleType: hc.Query("rule_type"),
		AgentID:  parseInt64Query(hc, "agent_id"),
		Page:     parseIntQueryDefault(hc, "page", 1),
		PageSize: parseIntQueryDefault(hc, "page_size", 20),
	}

	logs, total, err := c.guardrailService.ListLogs(&req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"items": logs,
		"total": total,
	}))
}
