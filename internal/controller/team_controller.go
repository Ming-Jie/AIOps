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

type TeamController struct {
	teamService *service.TeamService
	rbacService *service.RBACService
	jwtCfg      auth.JWTConfig
	userStore   storage.UserStore
}

func NewTeamController(teamService *service.TeamService, jwtCfg auth.JWTConfig, userStore storage.UserStore, rbacService ...*service.RBACService) *TeamController {
	ctrl := &TeamController{
		teamService: teamService,
		jwtCfg:      jwtCfg,
		userStore:   userStore,
	}
	if len(rbacService) > 0 {
		ctrl.rbacService = rbacService[0]
	}
	return ctrl
}

func (c *TeamController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/teams")
	if c.userStore != nil {
		g.Use(auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	}

	g.GET("", c.ListTeams)
	g.GET("/:id", c.GetTeam)
	g.POST("", c.CreateTeam)
	g.PUT("/:id", c.UpdateTeam)
	g.DELETE("/:id", c.DeleteTeam)
	g.POST("/:id/conversations", c.StartConversation)
	g.GET("/conversations/:convId", c.GetConversation)
	g.GET("/:id/conversations", c.ListConversations)
	g.POST("/conversations/:convId/messages", c.SendMessage)
}

func (c *TeamController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (c *TeamController) ListTeams(ctx context.Context, hc *app.RequestContext) {
	teams, err := c.teamService.ListTeams()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(teams))
}

func (c *TeamController) GetTeam(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid team id"))
		return
	}
	team, err := c.teamService.GetTeam(id)
	if err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(team))
}

func (c *TeamController) CreateTeam(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateTeamRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	user := auth.GetCurrentUser(hc)
	var userID int64
	if user != nil {
		userID = user.ID
	}
	team, err := c.teamService.CreateTeam(&req, userID)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusCreated, schema.SuccessResponse(team))
}

func (c *TeamController) UpdateTeam(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid team id"))
		return
	}

	var req schema.UpdateTeamRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	team, err := c.teamService.UpdateTeam(id, &req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(team))
}

func (c *TeamController) DeleteTeam(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid team id"))
		return
	}

	if err := c.teamService.DeleteTeam(id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

func (c *TeamController) StartConversation(ctx context.Context, hc *app.RequestContext) {
	teamID := parseInt64Param(hc, "id")
	if teamID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid team id"))
		return
	}

	title := hc.Query("title")
	if title == "" {
		title = "New Conversation"
	}

	user := auth.GetCurrentUser(hc)
	var userID int64
	if user != nil {
		userID = user.ID
	}
	conv, err := c.teamService.StartConversation(ctx, teamID, title, userID)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusCreated, schema.SuccessResponse(conv))
}

func (c *TeamController) GetConversation(ctx context.Context, hc *app.RequestContext) {
	convID := parseInt64Param(hc, "convId")
	if convID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid conversation id"))
		return
	}

	conv, err := c.teamService.GetConversation(ctx, convID)
	if err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(conv))
}

func (c *TeamController) ListConversations(ctx context.Context, hc *app.RequestContext) {
	teamID := parseInt64Param(hc, "id")
	if teamID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid team id"))
		return
	}

	convs, err := c.teamService.ListConversations(teamID)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(convs))
}

func (c *TeamController) SendMessage(ctx context.Context, hc *app.RequestContext) {
	convID := parseInt64Param(hc, "convId")
	if convID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid conversation id"))
		return
	}

	var req schema.SendTeamMessageRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	conv, err := c.teamService.SendMessage(ctx, convID, req.Text)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(conv))
}
