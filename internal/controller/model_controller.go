package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/fisk086/aiops/internal/auth"
	appmodel "github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
)

type ModelController struct {
	svc       *service.ModelService
	jwtCfg    auth.JWTConfig
	userStore storage.UserStore
}

func NewModelController(svc *service.ModelService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *ModelController {
	return &ModelController{svc: svc, jwtCfg: jwtCfg, userStore: userStore}
}

func (c *ModelController) RegisterRoutes(h *server.Hertz) {
	g := h.Group("/api/v1/models")
	g.Use(auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	g.GET("", c.List)
	g.POST("", c.Create)
	g.GET("/:id", c.Get)
	g.PUT("/:id", c.Update)
	g.DELETE("/:id", c.Delete)
}

func (c *ModelController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

type modelConfigJSON struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	ModelName string `json:"model"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key,omitempty"`
	IsActive  bool   `json:"is_active"`
	Purpose   string `json:"purpose"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func modelToJSON(m *appmodel.ModelConfig) modelConfigJSON {
	j := modelConfigJSON{
		ID:        m.ID,
		Name:      m.Name,
		Provider:  m.Provider,
		ModelName: m.ModelName,
		BaseURL:   m.BaseURL,
		IsActive:  m.IsActive,
		Purpose:   m.Purpose,
		CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: m.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if m.APIKey != "" {
		j.APIKey = maskKey(m.APIKey)
	}
	return j
}

func maskKey(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

func (c *ModelController) List(ctx context.Context, rctx *app.RequestContext) {
	list, err := c.svc.List(ctx)
	if err != nil {
		rctx.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*appmodel.ModelConfig{}
	}
	out := make([]modelConfigJSON, 0, len(list))
	for _, m := range list {
		out = append(out, modelToJSON(m))
	}
	rctx.JSON(http.StatusOK, utils.H{"data": out})
}

func (c *ModelController) Get(ctx context.Context, rctx *app.RequestContext) {
	id, err := strconv.ParseInt(rctx.Param("id"), 10, 64)
	if err != nil {
		rctx.JSON(http.StatusBadRequest, utils.H{"error": "invalid id"})
		return
	}
	m, err := c.svc.Get(ctx, id)
	if err != nil {
		rctx.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	if m == nil {
		rctx.JSON(http.StatusNotFound, utils.H{"error": "not found"})
		return
	}
	rctx.JSON(http.StatusOK, utils.H{"data": modelToJSON(m)})
}

type createModelRequest struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	ModelName string `json:"model"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	IsActive  *bool  `json:"is_active"`
	Purpose   string `json:"purpose"`
}

func (c *ModelController) Create(ctx context.Context, rctx *app.RequestContext) {
	var req createModelRequest
	if err := rctx.BindAndValidate(&req); err != nil {
		rctx.JSON(http.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	purpose := req.Purpose
	if purpose == "" {
		purpose = "chat"
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	m, err := c.svc.Create(ctx, &appmodel.ModelConfig{
		Name:      req.Name,
		Provider:  req.Provider,
		ModelName: req.ModelName,
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		IsActive:  isActive,
		Purpose:   purpose,
	})
	if err != nil {
		rctx.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	rctx.JSON(http.StatusCreated, utils.H{"data": modelToJSON(m)})
}

func (c *ModelController) Update(ctx context.Context, rctx *app.RequestContext) {
	id, err := strconv.ParseInt(rctx.Param("id"), 10, 64)
	if err != nil {
		rctx.JSON(http.StatusBadRequest, utils.H{"error": "invalid id"})
		return
	}
	var req createModelRequest
	if err := rctx.BindAndValidate(&req); err != nil {
		rctx.JSON(http.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	purpose := req.Purpose
	if purpose == "" {
		purpose = "chat"
	}
	m, err := c.svc.Update(ctx, id, &appmodel.ModelConfig{
		Name:      req.Name,
		Provider:  req.Provider,
		ModelName: req.ModelName,
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		IsActive:  req.IsActive != nil && *req.IsActive,
		Purpose:   purpose,
	})
	if err != nil {
		rctx.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	rctx.JSON(http.StatusOK, utils.H{"data": modelToJSON(m)})
}

func (c *ModelController) Delete(ctx context.Context, rctx *app.RequestContext) {
	id, err := strconv.ParseInt(rctx.Param("id"), 10, 64)
	if err != nil {
		rctx.JSON(http.StatusBadRequest, utils.H{"error": "invalid id"})
		return
	}
	if err := c.svc.Delete(ctx, id); err != nil {
		rctx.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}
	rctx.JSON(http.StatusOK, utils.H{"data": "deleted"})
}
