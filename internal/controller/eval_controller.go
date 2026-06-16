package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
)

type EvalController struct {
	svc       *service.EvalService
	jwtCfg    auth.JWTConfig
	userStore storage.UserStore
}

func NewEvalController(svc *service.EvalService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *EvalController {
	return &EvalController{svc: svc, jwtCfg: jwtCfg, userStore: userStore}
}

func (ctl *EvalController) ListCases(ctx context.Context, c *app.RequestContext) {
	agentID, _ := strconv.ParseInt(c.Query("agent_id"), 10, 64)
	cases, err := ctl.svc.ListCases(agentID)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(cases))
}

func (ctl *EvalController) GetCase(ctx context.Context, c *app.RequestContext) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ca, err := ctl.svc.GetCase(id)
	if err != nil {
		c.JSON(consts.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(ca))
}

func (ctl *EvalController) CreateCase(ctx context.Context, c *app.RequestContext) {
	var req schema.CreateEvalCaseRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	userID := int64(1)
	ca, err := ctl.svc.CreateCase(&req, userID)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusCreated, schema.SuccessResponse(ca))
}

func (ctl *EvalController) UpdateCase(ctx context.Context, c *app.RequestContext) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req schema.UpdateEvalCaseRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	ca, err := ctl.svc.UpdateCase(id, &req)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(ca))
}

func (ctl *EvalController) DeleteCase(ctx context.Context, c *app.RequestContext) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := ctl.svc.DeleteCase(id); err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctl *EvalController) StartRun(ctx context.Context, c *app.RequestContext) {
	var req schema.StartEvalRunRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	userID := int64(1)
	run, err := ctl.svc.StartRun(ctx, &req, userID)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusCreated, schema.SuccessResponse(run))
}

func (ctl *EvalController) GetRun(ctx context.Context, c *app.RequestContext) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	run, err := ctl.svc.GetRun(ctx, id)
	if err != nil {
		c.JSON(consts.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(run))
}

func (ctl *EvalController) ListRuns(ctx context.Context, c *app.RequestContext) {
	agentID, _ := strconv.ParseInt(c.Query("agent_id"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	runs, err := ctl.svc.ListRuns(agentID, limit)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(runs))
}

func (ctl *EvalController) GetStats(ctx context.Context, c *app.RequestContext) {
	stats, err := ctl.svc.GetStats()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(stats))
}

func (ctl *EvalController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/eval")
	if ctl.userStore != nil {
		g.Use(auth.JWTMiddleware(ctl.jwtCfg, ctl.getUserForMiddleware))
	}
	g.GET("/cases", ctl.ListCases)
	g.GET("/cases/:id", ctl.GetCase)
	g.POST("/cases", ctl.CreateCase)
	g.PUT("/cases/:id", ctl.UpdateCase)
	g.DELETE("/cases/:id", ctl.DeleteCase)
	g.POST("/runs", ctl.StartRun)
	g.GET("/runs", ctl.ListRuns)
	g.GET("/runs/:id", ctl.GetRun)
	g.GET("/stats", ctl.GetStats)
}

func (ctl *EvalController) getUserForMiddleware(userID int64) (*auth.User, error) {
	user, err := ctl.userStore.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &auth.User{ID: user.ID, Username: user.Username, Email: user.Email, Status: string(user.Status), IsAdmin: user.IsAdmin}, nil
}
