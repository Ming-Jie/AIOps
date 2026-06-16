package controller

import (
	"context"
	"strconv"

	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type MessageController struct {
	messageService *service.MessageService
	jwtCfg         auth.JWTConfig
	userStore      storage.UserStore
}

func NewMessageController(messageService *service.MessageService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *MessageController {
	return &MessageController{messageService: messageService, jwtCfg: jwtCfg, userStore: userStore}
}

func (ctrl *MessageController) RegisterRoutes(h *server.Hertz) {
	g := h.Group("/api/v1")
	if ctrl.userStore != nil {
		g.Use(auth.JWTMiddleware(ctrl.jwtCfg, ctrl.getUserForMiddleware))
	}

	g.POST("/messages/send", ctrl.SendMessage)
	g.POST("/messages/span", ctrl.SendSpan)
	g.GET("/messages", ctrl.ListMessages)

	g.POST("/message-channels", ctrl.CreateChannel)
	g.GET("/message-channels", ctrl.ListChannels)
	g.GET("/message-channels/:id", ctrl.GetChannel)
	g.PUT("/message-channels/:id", ctrl.UpdateChannel)
	g.DELETE("/message-channels/:id", ctrl.DeleteChannel)

	g.POST("/a2a-cards", ctrl.CreateA2ACard)
	g.GET("/a2a-cards", ctrl.ListA2ACards)
	g.GET("/a2a-cards/:id", ctrl.GetA2ACard)
	g.PUT("/a2a-cards/:id", ctrl.UpdateA2ACard)
	g.DELETE("/a2a-cards/:id", ctrl.DeleteA2ACard)
}

func (ctrl *MessageController) getUserForMiddleware(userID int64) (*auth.User, error) {
	user, err := ctrl.userStore.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &auth.User{ID: user.ID, Username: user.Username, Email: user.Email, Status: string(user.Status), IsAdmin: user.IsAdmin}, nil
}

func (ctrl *MessageController) SendMessage(c context.Context, ctx *app.RequestContext) {
	var req schema.SendMessageRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	resp, err := ctrl.messageService.SendMessage(c, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(resp))
}

func (ctrl *MessageController) SendSpan(c context.Context, ctx *app.RequestContext) {
	var req schema.MessageSpanRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	resp, err := ctrl.messageService.SendSpan(c, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(resp))
}

func (ctrl *MessageController) ListMessages(c context.Context, ctx *app.RequestContext) {
	req := schema.ListMessagesRequest{
		Limit:  parseIntQueryDefault(ctx, "limit", 50),
		Offset: parseIntQueryDefault(ctx, "offset", 0),
	}

	if ch := ctx.Query("channel_id"); ch != "" {
		if id, err := strconv.ParseInt(ch, 10, 64); err == nil {
			req.ChannelID = id
		}
	}
	if ag := ctx.Query("agent_id"); ag != "" {
		if id, err := strconv.ParseInt(ag, 10, 64); err == nil {
			req.AgentID = id
		}
	}
	if sid := ctx.Query("session_id"); sid != "" {
		req.SessionID = sid
	}
	if st := ctx.Query("status"); st != "" {
		req.Status = st
	}

	messages, total, err := ctrl.messageService.ListMessages(&req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(map[string]any{
		"messages": messages,
		"total":    total,
		"limit":    req.Limit,
		"offset":   req.Offset,
	}))
}

func (ctrl *MessageController) CreateChannel(c context.Context, ctx *app.RequestContext) {
	var req schema.CreateMessageChannelRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	ch, err := ctrl.messageService.CreateChannel(&req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(ch))
}

func (ctrl *MessageController) ListChannels(c context.Context, ctx *app.RequestContext) {
	agentID := parseInt64Query(ctx, "agent_id")

	channels, err := ctrl.messageService.ListChannels(agentID)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(channels))
}

func (ctrl *MessageController) GetChannel(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	ch, err := ctrl.messageService.GetChannel(id)
	if err != nil {
		ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("channel not found"))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(ch))
}

func (ctrl *MessageController) UpdateChannel(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	var req schema.UpdateMessageChannelRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	ch, err := ctrl.messageService.UpdateChannel(id, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(ch))
}

func (ctrl *MessageController) DeleteChannel(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	if err := ctrl.messageService.DeleteChannel(id); err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *MessageController) CreateA2ACard(c context.Context, ctx *app.RequestContext) {
	var req schema.CreateA2ACardRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	card, err := ctrl.messageService.CreateA2ACard(&req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(card))
}

func (ctrl *MessageController) ListA2ACards(c context.Context, ctx *app.RequestContext) {
	agentID := parseInt64Query(ctx, "agent_id")

	cards, err := ctrl.messageService.ListA2ACards(agentID)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(cards))
}

func (ctrl *MessageController) GetA2ACard(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	card, err := ctrl.messageService.GetA2ACard(id)
	if err != nil {
		ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("card not found"))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(card))
}

func (ctrl *MessageController) UpdateA2ACard(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	var req schema.UpdateA2ACardRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	card, err := ctrl.messageService.UpdateA2ACard(id, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(card))
}

func (ctrl *MessageController) DeleteA2ACard(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	if err := ctrl.messageService.DeleteA2ACard(id); err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}
