package service

import (
	"context"
	"time"

	"github.com/fisk086/aiops/internal/guard"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/schema"
	"gorm.io/gorm"
)

type GuardrailStore interface {
	CreateRule(rule *model.GuardrailRule) (*model.GuardrailRule, error)
	UpdateRule(id int64, rule *model.GuardrailRule) (*model.GuardrailRule, error)
	GetRule(id int64) (*model.GuardrailRule, error)
	ListRules() ([]*model.GuardrailRule, error)
	DeleteRule(id int64) error
	GetActiveRulesByAgent(ctx context.Context, agentID int64) ([]*model.GuardrailRule, error)
	GetRuleByID(ctx context.Context, id int64) (*model.GuardrailRule, error)
	CreateLog(ctx context.Context, log *model.GuardrailLog) error
	ListLogs(req *schema.ListGuardrailLogsRequest) ([]*model.GuardrailLog, int64, error)
	IncrementRuleHit(ctx context.Context, ruleID int64) error
	SetAgentBindings(ruleID int64, agentIDs []int64) error
	GetAgentBindings(ruleID int64) ([]*model.GuardrailAgentBinding, error)
}

type guardrailGORMStore struct {
	db *gorm.DB
}

func NewGuardrailStore(db *gorm.DB) GuardrailStore {
	return &guardrailGORMStore{db: db}
}

func (s *guardrailGORMStore) CreateRule(rule *model.GuardrailRule) (*model.GuardrailRule, error) {
	if err := s.db.Create(rule).Error; err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *guardrailGORMStore) UpdateRule(id int64, rule *model.GuardrailRule) (*model.GuardrailRule, error) {
	var existing model.GuardrailRule
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&existing).Updates(rule).Error; err != nil {
		return nil, err
	}
	s.db.First(&existing, id)
	return &existing, nil
}

func (s *guardrailGORMStore) GetRule(id int64) (*model.GuardrailRule, error) {
	var rule model.GuardrailRule
	if err := s.db.First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *guardrailGORMStore) ListRules() ([]*model.GuardrailRule, error) {
	var rules []*model.GuardrailRule
	if err := s.db.Order("priority desc, id asc").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *guardrailGORMStore) DeleteRule(id int64) error {
	s.db.Where("rule_id = ?", id).Delete(&model.GuardrailAgentBinding{})
	return s.db.Delete(&model.GuardrailRule{}, id).Error
}

func (s *guardrailGORMStore) GetActiveRulesByAgent(ctx context.Context, agentID int64) ([]*model.GuardrailRule, error) {
	var rules []*model.GuardrailRule
	err := s.db.Raw(`
		SELECT r.* FROM guardrail_rules r
		INNER JOIN guardrail_agent_bindings b ON b.rule_id = r.id
		WHERE b.agent_id = ? AND r.is_active = true AND b.is_active = true
		ORDER BY r.priority DESC, r.id ASC
	`, agentID).Scan(&rules).Error
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *guardrailGORMStore) GetRuleByID(ctx context.Context, id int64) (*model.GuardrailRule, error) {
	return s.GetRule(id)
}

func (s *guardrailGORMStore) CreateLog(ctx context.Context, log *model.GuardrailLog) error {
	return s.db.WithContext(ctx).Create(log).Error
}

func (s *guardrailGORMStore) ListLogs(req *schema.ListGuardrailLogsRequest) ([]*model.GuardrailLog, int64, error) {
	query := s.db.Model(&model.GuardrailLog{})
	if req.RuleType != "" {
		query = query.Where("rule_type = ?", req.RuleType)
	}
	if req.AgentID > 0 {
		query = query.Where("agent_id = ?", req.AgentID)
	}
	if req.Blocked != nil {
		query = query.Where("blocked = ?", *req.Blocked)
	}

	var total int64
	query.Count(&total)

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var logs []*model.GuardrailLog
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (s *guardrailGORMStore) IncrementRuleHit(ctx context.Context, ruleID int64) error {
	return s.db.Model(&model.GuardrailRule{}).Where("id = ?", ruleID).
		Updates(map[string]interface{}{
			"hit_count":  gorm.Expr("hit_count + 1"),
			"last_hit_at": time.Now(),
		}).Error
}

func (s *guardrailGORMStore) SetAgentBindings(ruleID int64, agentIDs []int64) error {
	s.db.Where("rule_id = ?", ruleID).Delete(&model.GuardrailAgentBinding{})
	if len(agentIDs) == 0 {
		return nil
	}
	bindings := make([]model.GuardrailAgentBinding, len(agentIDs))
	for i, aid := range agentIDs {
		bindings[i] = model.GuardrailAgentBinding{RuleID: ruleID, AgentID: aid}
	}
	return s.db.Create(&bindings).Error
}

func (s *guardrailGORMStore) GetAgentBindings(ruleID int64) ([]*model.GuardrailAgentBinding, error) {
	var bindings []*model.GuardrailAgentBinding
	if err := s.db.Where("rule_id = ?", ruleID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

type GuardrailService struct {
	store  GuardrailStore
	engine *guard.Engine
}

func NewGuardrailService(store GuardrailStore) *GuardrailService {
	svc := &GuardrailService{store: store}
	svc.engine = guard.NewEngine(store)
	return svc
}

func (s *GuardrailService) ListRules() ([]*schema.GuardrailRuleResponse, error) {
	rules, err := s.store.ListRules()
	if err != nil {
		return nil, err
	}
	result := make([]*schema.GuardrailRuleResponse, 0, len(rules))
	for _, r := range rules {
		result = append(result, s.toRuleResponse(r))
	}
	return result, nil
}

func (s *GuardrailService) GetRule(id int64) (*schema.GuardrailRuleResponse, error) {
	rule, err := s.store.GetRule(id)
	if err != nil {
		return nil, err
	}
	return s.toRuleResponse(rule), nil
}

func (s *GuardrailService) CreateRule(req *schema.CreateGuardrailRuleRequest, userID int64) (*schema.GuardrailRuleResponse, error) {
	rule := &model.GuardrailRule{
		Name:        req.Name,
		Description: req.Description,
		RuleType:    req.RuleType,
		Scope:       model.GuardrailRuleScope(getOrDefault(req.Scope, "both")),
		Action:      model.GuardrailAction(getOrDefault(req.Action, "block")),
		Severity:    getOrDefault(req.Severity, "medium"),
		IsActive:    true,
		Config:      req.Config,
		CreatedBy:   userID,
	}

	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.IsActive != nil {
		rule.IsActive = *req.IsActive
	}

	created, err := s.store.CreateRule(rule)
	if err != nil {
		return nil, err
	}

	if len(req.AgentIDs) > 0 {
		if err := s.store.SetAgentBindings(created.ID, req.AgentIDs); err != nil {
			logger.Error("failed to set agent bindings", "rule_id", created.ID, "err", err)
		}
	}

	return s.toRuleResponse(created), nil
}

func (s *GuardrailService) UpdateRule(id int64, req *schema.UpdateGuardrailRuleRequest) (*schema.GuardrailRuleResponse, error) {
	rule, err := s.store.GetRule(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.RuleType != nil {
		rule.RuleType = *req.RuleType
	}
	if req.Scope != nil {
		rule.Scope = model.GuardrailRuleScope(*req.Scope)
	}
	if req.Action != nil {
		rule.Action = model.GuardrailAction(*req.Action)
	}
	if req.Severity != nil {
		rule.Severity = *req.Severity
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.IsActive != nil {
		rule.IsActive = *req.IsActive
	}
	if req.Config != nil {
		rule.Config = req.Config
	}

	updated, err := s.store.UpdateRule(id, rule)
	if err != nil {
		return nil, err
	}

	if req.AgentIDs != nil {
		s.store.SetAgentBindings(id, req.AgentIDs)
	}

	return s.toRuleResponse(updated), nil
}

func (s *GuardrailService) DeleteRule(id int64) error {
	return s.store.DeleteRule(id)
}

func (s *GuardrailService) ListLogs(req *schema.ListGuardrailLogsRequest) ([]*schema.GuardrailLogResponse, int64, error) {
	logs, total, err := s.store.ListLogs(req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*schema.GuardrailLogResponse, 0, len(logs))
	for _, l := range logs {
		result = append(result, &schema.GuardrailLogResponse{
			ID:        l.ID,
			RuleID:    l.RuleID,
			RuleName:  l.RuleName,
			RuleType:  l.RuleType,
			AgentID:   l.AgentID,
			Scope:     l.Scope,
			Action:    string(l.Action),
			Severity:  l.Severity,
			UserID:    l.UserID,
			SessionID: l.SessionID,
			Input:     l.Input,
			Output:    l.Output,
			MatchInfo: l.MatchInfo,
			Blocked:   l.Blocked,
			CreatedAt: l.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, total, nil
}

func (s *GuardrailService) TestRule(req *schema.TestGuardrailRequest) (*schema.TestGuardrailResponse, error) {
	if req.RuleID > 0 {
		rule, err := s.store.GetRuleByID(context.Background(), req.RuleID)
		if err != nil {
			return nil, err
		}
		result := s.engine.TestRule(rule, req.Text)
		if result != nil {
			return &schema.TestGuardrailResponse{
				Triggered: result.Triggered,
				RuleName:  rule.Name,
				RuleType:  rule.RuleType,
				Action:    string(result.Action),
				MatchInfo: result.MatchInfo,
			}, nil
		}
		return &schema.TestGuardrailResponse{Triggered: false}, nil
	}

	allRules, err := s.store.ListRules()
	if err != nil {
		return nil, err
	}

	for _, rule := range allRules {
		if rule.Scope != "both" && string(rule.Scope) != req.Scope {
			continue
		}
		result := s.engine.TestRule(rule, req.Text)
		if result != nil && result.Triggered {
			return &schema.TestGuardrailResponse{
				Triggered: true,
				RuleName:  rule.Name,
				RuleType:  rule.RuleType,
				Action:    string(result.Action),
				MatchInfo: result.MatchInfo,
			}, nil
		}
	}

	return &schema.TestGuardrailResponse{Triggered: false}, nil
}

func (s *GuardrailService) CheckInput(ctx context.Context, text string, agentID int64, userID int64, sessionID string) *guard.Result {
	return s.engine.CheckInput(ctx, text, agentID, userID, sessionID)
}

func (s *GuardrailService) CheckOutput(ctx context.Context, text string, agentID int64, userID int64, sessionID string) *guard.Result {
	return s.engine.CheckOutput(ctx, text, agentID, userID, sessionID)
}

func (s *GuardrailService) toRuleResponse(rule *model.GuardrailRule) *schema.GuardrailRuleResponse {
	resp := &schema.GuardrailRuleResponse{
		ID:          rule.ID,
		Name:        rule.Name,
		Description: rule.Description,
		RuleType:    rule.RuleType,
		Scope:       string(rule.Scope),
		Action:      string(rule.Action),
		Severity:    rule.Severity,
		Priority:    rule.Priority,
		IsActive:    rule.IsActive,
		Config:      rule.Config,
		HitCount:    rule.HitCount,
		CreatedBy:   rule.CreatedBy,
		CreatedAt:   rule.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   rule.UpdatedAt.Format(time.RFC3339),
	}
	if rule.LastHitAt != nil {
		s := rule.LastHitAt.Format(time.RFC3339)
		resp.LastHitAt = &s
	}

	bindings, err := s.store.GetAgentBindings(rule.ID)
	if err == nil && len(bindings) > 0 {
		resp.BoundAgents = make([]*schema.AgentBrief, 0, len(bindings))
		for _, b := range bindings {
			resp.BoundAgents = append(resp.BoundAgents, &schema.AgentBrief{ID: b.AgentID})
		}
	}

	return resp
}

func getOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
