package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
)

type IMBotHistoryController struct {
	imHistory *service.IMHistoryService
	jwtCfg    auth.JWTConfig
	userStore storage.UserStore
}

func NewIMBotHistoryController(imHistory *service.IMHistoryService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *IMBotHistoryController {
	return &IMBotHistoryController{imHistory: imHistory, jwtCfg: jwtCfg, userStore: userStore}
}

func (c *IMBotHistoryController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/imbots")
	g.Use(auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	g.GET("/sessions", c.ListSessions)
	g.GET("/sessions/:sessionId/messages", c.ListMessages)
}

func (c *IMBotHistoryController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (c *IMBotHistoryController) ListSessions(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Query(hc, "agent_id")
	if agentID < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent_id is required and must be >= 1"))
		return
	}
	channel := hc.Query("channel")
	limit := parseIntQueryDefault(hc, "limit", 50)
	offset := parseIntQueryDefault(hc, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	list, err := c.imHistory.ListIMSessions(ctx, agentID, channel, limit, offset)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(list))
}

func (c *IMBotHistoryController) ListMessages(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Query(hc, "agent_id")
	if agentID < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent_id is required and must be >= 1"))
		return
	}
	sessionID := hc.Param("sessionId")
	if sessionID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id required"))
		return
	}
	limit := parseIntQueryDefault(hc, "limit", 100)
	var msgs []schema.ChatHistoryMessage
	var err error
	if hc.Query("offset") == "" {
		msgs, err = c.imHistory.ListIMMessages(ctx, agentID, sessionID, limit, 0)
	} else {
		offset := parseIntQueryDefault(hc, "offset", 0)
		if offset < 0 {
			offset = 0
		}
		msgs, err = c.imHistory.ListIMMessages(ctx, agentID, sessionID, limit, offset)
	}
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("session not found"))
			return
		}
		if errors.Is(err, storage.ErrSessionForbidden) {
			hc.JSON(http.StatusForbidden, schema.ErrorResponse("forbidden"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(msgs))
}
