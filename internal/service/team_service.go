package service

import (
	"context"
	"fmt"
	"time"

	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/orchestrator"
	"github.com/fisk086/aiops/internal/schema"
	"gorm.io/gorm"
)

type TeamStore interface {
	CreateTeam(team *model.Team) (*model.Team, error)
	UpdateTeam(id int64, team *model.Team) (*model.Team, error)
	GetTeam(id int64) (*model.Team, error)
	GetTeamWithMembers(ctx context.Context, teamID int64) (*model.Team, []*model.TeamMember, error)
	ListTeams() ([]*model.Team, error)
	DeleteTeam(id int64) error
	SetTeamMembers(teamID int64, agentIDs []int64) error
	GetTeamMembers(teamID int64) ([]*model.TeamMember, error)
	CreateConversation(ctx context.Context, conv *model.TeamConversation) (*model.TeamConversation, error)
	GetConversation(ctx context.Context, id int64) (*model.TeamConversation, error)
	UpdateConversation(ctx context.Context, conv *model.TeamConversation) error
	ListConversations(teamID int64) ([]*model.TeamConversation, error)
	CreateMessage(ctx context.Context, msg *model.TeamMessage) (*model.TeamMessage, error)
	ListMessagesByConversation(ctx context.Context, convID int64) ([]*model.TeamMessage, error)
}

type teamGORMStore struct {
	db *gorm.DB
}

func NewTeamStore(db *gorm.DB) TeamStore {
	return &teamGORMStore{db: db}
}

func (s *teamGORMStore) CreateTeam(team *model.Team) (*model.Team, error) {
	if err := s.db.Create(team).Error; err != nil {
		return nil, err
	}
	return team, nil
}

func (s *teamGORMStore) UpdateTeam(id int64, team *model.Team) (*model.Team, error) {
	var existing model.Team
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&existing).Updates(team).Error; err != nil {
		return nil, err
	}
	s.db.First(&existing, id)
	return &existing, nil
}

func (s *teamGORMStore) GetTeam(id int64) (*model.Team, error) {
	var team model.Team
	if err := s.db.First(&team, id).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (s *teamGORMStore) GetTeamWithMembers(ctx context.Context, teamID int64) (*model.Team, []*model.TeamMember, error) {
	team, err := s.GetTeam(teamID)
	if err != nil {
		return nil, nil, err
	}
	members, err := s.GetTeamMembers(teamID)
	if err != nil {
		return nil, nil, err
	}
	return team, members, nil
}

func (s *teamGORMStore) ListTeams() ([]*model.Team, error) {
	var teams []*model.Team
	if err := s.db.Order("id asc").Find(&teams).Error; err != nil {
		return nil, err
	}
	return teams, nil
}

func (s *teamGORMStore) DeleteTeam(id int64) error {
	s.db.Where("team_id = ?", id).Delete(&model.TeamMember{})
	s.db.Where("team_id = ?", id).Delete(&model.TeamMessage{})
	s.db.Where("team_id = ?", id).Delete(&model.TeamConversation{})
	return s.db.Delete(&model.Team{}, id).Error
}

func (s *teamGORMStore) SetTeamMembers(teamID int64, agentIDs []int64) error {
	s.db.Where("team_id = ?", teamID).Delete(&model.TeamMember{})
	if len(agentIDs) == 0 {
		return nil
	}
	members := make([]model.TeamMember, len(agentIDs))
	for i, aid := range agentIDs {
		members[i] = model.TeamMember{
			TeamID:    teamID,
			AgentID:   aid,
			SortOrder: i,
		}
	}
	return s.db.Create(&members).Error
}

func (s *teamGORMStore) GetTeamMembers(teamID int64) ([]*model.TeamMember, error) {
	var members []*model.TeamMember
	if err := s.db.Where("team_id = ?", teamID).Order("sort_order asc").Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *teamGORMStore) CreateConversation(ctx context.Context, conv *model.TeamConversation) (*model.TeamConversation, error) {
	if err := s.db.WithContext(ctx).Create(conv).Error; err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *teamGORMStore) GetConversation(ctx context.Context, id int64) (*model.TeamConversation, error) {
	var conv model.TeamConversation
	if err := s.db.WithContext(ctx).First(&conv, id).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (s *teamGORMStore) UpdateConversation(ctx context.Context, conv *model.TeamConversation) error {
	return s.db.WithContext(ctx).Save(conv).Error
}

func (s *teamGORMStore) ListConversations(teamID int64) ([]*model.TeamConversation, error) {
	var convs []*model.TeamConversation
	if err := s.db.Where("team_id = ?", teamID).Order("created_at desc").Find(&convs).Error; err != nil {
		return nil, err
	}
	return convs, nil
}

func (s *teamGORMStore) CreateMessage(ctx context.Context, msg *model.TeamMessage) (*model.TeamMessage, error) {
	if err := s.db.WithContext(ctx).Create(msg).Error; err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *teamGORMStore) ListMessagesByConversation(ctx context.Context, convID int64) ([]*model.TeamMessage, error) {
	var msgs []*model.TeamMessage
	if err := s.db.WithContext(ctx).Where("conversation_id = ?", convID).Order("created_at asc").Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

type TeamService struct {
	store         TeamStore
	agentService  *AgentService
	orchestrator  *orchestrator.Orchestrator
}

func NewTeamService(store TeamStore, agentService *AgentService, orch *orchestrator.Orchestrator) *TeamService {
	return &TeamService{store: store, agentService: agentService, orchestrator: orch}
}

func (s *TeamService) ListTeams() ([]*schema.TeamResponse, error) {
	teams, err := s.store.ListTeams()
	if err != nil {
		return nil, err
	}
	result := make([]*schema.TeamResponse, 0, len(teams))
	for _, t := range teams {
		result = append(result, s.toTeamResponse(t))
	}
	return result, nil
}

func (s *TeamService) GetTeam(id int64) (*schema.TeamResponse, error) {
	team, err := s.store.GetTeam(id)
	if err != nil {
		return nil, err
	}
	return s.toTeamResponse(team), nil
}

func (s *TeamService) CreateTeam(req *schema.CreateTeamRequest, userID int64) (*schema.TeamResponse, error) {
	mode := model.TeamCollaborationMode(getOrDefault(req.Mode, string(model.TeamModeGroupChat)))

	team := &model.Team{
		Name:               req.Name,
		Description:        req.Description,
		Mode:               mode,
		CoordinatorAgentID: req.CoordinatorAgentID,
		MaxRounds:          5,
		IsActive:           true,
		Config:             req.Config,
		CreatedBy:          userID,
	}

	if req.MaxRounds != nil {
		team.MaxRounds = *req.MaxRounds
	}
	if req.IsActive != nil {
		team.IsActive = *req.IsActive
	}

	created, err := s.store.CreateTeam(team)
	if err != nil {
		return nil, err
	}

	if len(req.AgentIDs) > 0 {
		if err := s.store.SetTeamMembers(created.ID, req.AgentIDs); err != nil {
			logger.Error("failed to set team members", "team_id", created.ID, "err", err)
		}
	}

	return s.toTeamResponse(created), nil
}

func (s *TeamService) UpdateTeam(id int64, req *schema.UpdateTeamRequest) (*schema.TeamResponse, error) {
	team, err := s.store.GetTeam(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		team.Name = *req.Name
	}
	if req.Description != nil {
		team.Description = *req.Description
	}
	if req.Mode != nil {
		team.Mode = model.TeamCollaborationMode(*req.Mode)
	}
	if req.CoordinatorAgentID != nil {
		team.CoordinatorAgentID = req.CoordinatorAgentID
	}
	if req.MaxRounds != nil {
		team.MaxRounds = *req.MaxRounds
	}
	if req.IsActive != nil {
		team.IsActive = *req.IsActive
	}
	if req.Config != nil {
		team.Config = req.Config
	}

	updated, err := s.store.UpdateTeam(id, team)
	if err != nil {
		return nil, err
	}

	if req.AgentIDs != nil {
		s.store.SetTeamMembers(id, req.AgentIDs)
	}

	return s.toTeamResponse(updated), nil
}

func (s *TeamService) DeleteTeam(id int64) error {
	return s.store.DeleteTeam(id)
}

func (s *TeamService) StartConversation(ctx context.Context, teamID int64, title string, userID int64) (*schema.TeamConversationResponse, error) {
	conv, err := s.orchestrator.StartConversation(ctx, teamID, title, userID)
	if err != nil {
		return nil, err
	}
	return s.toConversationResponse(conv, nil), nil
}

func (s *TeamService) GetConversation(ctx context.Context, id int64) (*schema.TeamConversationResponse, error) {
	conv, err := s.store.GetConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	msgs, err := s.store.ListMessagesByConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := s.toConversationResponse(conv, msgs)
	s.expandConversationWebImages(ctx, resp)
	return resp, nil
}

func (s *TeamService) ListConversations(teamID int64) ([]*schema.TeamConversationResponse, error) {
	convs, err := s.store.ListConversations(teamID)
	if err != nil {
		return nil, err
	}
	result := make([]*schema.TeamConversationResponse, 0, len(convs))
	for _, c := range convs {
		result = append(result, s.toConversationResponse(c, nil))
	}
	return result, nil
}

func (s *TeamService) SendMessage(ctx context.Context, convID int64, text string) (*schema.TeamConversationResponse, error) {
	resp, err := s.orchestrator.SendMessage(ctx, convID, text)
	if resp != nil {
		s.expandConversationWebImages(ctx, resp)
	}
	return resp, err
}

func (s *TeamService) expandConversationWebImages(ctx context.Context, resp *schema.TeamConversationResponse) {
	if resp == nil || resp.ID < 1 || resp.TeamID < 1 || len(resp.Messages) == 0 {
		return
	}
	_, members, err := s.store.GetTeamWithMembers(ctx, resp.TeamID)
	if err != nil {
		return
	}
	agentIDs := teamMemberAgentIDs(members)
	seenFiles := make(map[string]struct{})
	for _, msg := range resp.Messages {
		if msg == nil {
			continue
		}
		msg.Content = expandTeamImagesForWeb(msg.Content, resp.ID, agentIDs, seenFiles)
	}
}

func (s *TeamService) RegisterAgent(awr *schema.AgentWithRuntime) {
	s.orchestrator.RegisterAgent(awr)
}

func (s *TeamService) UnregisterAgent(agentID int64) {
	s.orchestrator.UnregisterAgent(agentID)
}

func (s *TeamService) toTeamResponse(team *model.Team) *schema.TeamResponse {
	resp := &schema.TeamResponse{
		ID:                 team.ID,
		Name:               team.Name,
		Description:        team.Description,
		Mode:               string(team.Mode),
		CoordinatorAgentID: team.CoordinatorAgentID,
		MaxRounds:          team.MaxRounds,
		IsActive:           team.IsActive,
		Config:             team.Config,
		CreatedBy:          team.CreatedBy,
		CreatedAt:          team.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          team.UpdatedAt.Format(time.RFC3339),
	}

	members, err := s.store.GetTeamMembers(team.ID)
	if err == nil {
		resp.Members = make([]*schema.TeamMemberResp, 0, len(members))
		for _, m := range members {
			agentName := fmt.Sprintf("Agent(%d)", m.AgentID)
			if agent, err := s.agentService.GetAgent(m.AgentID); err == nil && agent != nil {
				agentName = agent.Name
			}
			resp.Members = append(resp.Members, &schema.TeamMemberResp{
				ID:        m.ID,
				TeamID:    m.TeamID,
				AgentID:   m.AgentID,
				AgentName: agentName,
				Role:      m.Role,
				SortOrder: m.SortOrder,
			})
		}
	}

	return resp
}

func (s *TeamService) toConversationResponse(conv *model.TeamConversation, msgs []*model.TeamMessage) *schema.TeamConversationResponse {
	resp := &schema.TeamConversationResponse{
		ID:        conv.ID,
		TeamID:    conv.TeamID,
		Title:     conv.Title,
		Status:    conv.Status,
		StartedBy: conv.StartedBy,
		Round:     conv.Round,
		CreatedAt: conv.CreatedAt.Format(time.RFC3339),
		UpdatedAt: conv.UpdatedAt.Format(time.RFC3339),
	}

	for _, m := range msgs {
		msg := &schema.TeamMsgResp{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			SenderAgentID:  m.SenderAgentID,
			SenderName:     m.SenderName,
			Content:        m.Content,
			MsgType:        m.MsgType,
			TargetAgentID:  m.TargetAgentID,
			Round:          m.Round,
			Metadata:       m.Metadata,
			CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		}
		resp.Messages = append(resp.Messages, msg)
	}

	return resp
}
