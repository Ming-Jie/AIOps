package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/schema"
	"gorm.io/gorm"
)

type EvalStore interface {
	CreateCase(c *model.EvalCase) (*model.EvalCase, error)
	UpdateCase(id int64, c *model.EvalCase) (*model.EvalCase, error)
	GetCase(id int64) (*model.EvalCase, error)
	ListCases(agentID int64) ([]*model.EvalCase, error)
	DeleteCase(id int64) error
	CreateRun(r *model.EvalRun) (*model.EvalRun, error)
	UpdateRun(r *model.EvalRun) error
	GetRun(id int64) (*model.EvalRun, error)
	ListRuns(agentID int64, limit int) ([]*model.EvalRun, error)
	CreateResult(r *model.EvalResult) (*model.EvalResult, error)
	ListResultsByRun(runID int64) ([]*model.EvalResult, error)
	GetStats() (totalCases int, totalRuns int, avgScore float64, bestScore float64, totalPassed int, totalFailed int, err error)
}

type evalGORMStore struct {
	db *gorm.DB
}

func NewEvalStore(db *gorm.DB) EvalStore {
	return &evalGORMStore{db: db}
}

func (s *evalGORMStore) CreateCase(c *model.EvalCase) (*model.EvalCase, error) {
	if err := s.db.Create(c).Error; err != nil {
		return nil, err
	}
	return c, nil
}

func (s *evalGORMStore) UpdateCase(id int64, c *model.EvalCase) (*model.EvalCase, error) {
	var existing model.EvalCase
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&existing).Updates(c).Error; err != nil {
		return nil, err
	}
	s.db.First(&existing, id)
	return &existing, nil
}

func (s *evalGORMStore) GetCase(id int64) (*model.EvalCase, error) {
	var c model.EvalCase
	if err := s.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *evalGORMStore) ListCases(agentID int64) ([]*model.EvalCase, error) {
	var cases []*model.EvalCase
	q := s.db.Where("is_active = ?", true)
	if agentID > 0 {
		q = q.Where("agent_id = ?", agentID)
	}
	if err := q.Order("id asc").Find(&cases).Error; err != nil {
		return nil, err
	}
	return cases, nil
}

func (s *evalGORMStore) DeleteCase(id int64) error {
	return s.db.Delete(&model.EvalCase{}, id).Error
}

func (s *evalGORMStore) CreateRun(r *model.EvalRun) (*model.EvalRun, error) {
	if err := s.db.Create(r).Error; err != nil {
		return nil, err
	}
	return r, nil
}

func (s *evalGORMStore) UpdateRun(r *model.EvalRun) error {
	return s.db.Save(r).Error
}

func (s *evalGORMStore) GetRun(id int64) (*model.EvalRun, error) {
	var r model.EvalRun
	if err := s.db.First(&r, id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *evalGORMStore) ListRuns(agentID int64, limit int) ([]*model.EvalRun, error) {
	var runs []*model.EvalRun
	q := s.db
	if agentID > 0 {
		q = q.Where("agent_id = ?", agentID)
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if err := q.Order("created_at desc").Limit(limit).Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *evalGORMStore) CreateResult(r *model.EvalResult) (*model.EvalResult, error) {
	if err := s.db.Create(r).Error; err != nil {
		return nil, err
	}
	return r, nil
}

func (s *evalGORMStore) ListResultsByRun(runID int64) ([]*model.EvalResult, error) {
	var results []*model.EvalResult
	if err := s.db.Where("run_id = ?", runID).Order("id asc").Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (s *evalGORMStore) GetStats() (int, int, float64, float64, int, int, error) {
	var totalCases int64
	s.db.Model(&model.EvalCase{}).Where("is_active = ?", true).Count(&totalCases)

	var totalRuns int64
	s.db.Model(&model.EvalRun{}).Count(&totalRuns)

	var avgScore, bestScore float64
	s.db.Model(&model.EvalRun{}).Select("COALESCE(AVG(score), 0)").Scan(&avgScore)
	s.db.Model(&model.EvalRun{}).Select("COALESCE(MAX(score), 0)").Scan(&bestScore)

	var totalPassed, totalFailed int64
	s.db.Model(&model.EvalResult{}).Where("passed = ?", true).Count(&totalPassed)
	s.db.Model(&model.EvalResult{}).Where("passed = ?", false).Count(&totalFailed)

	return int(totalCases), int(totalRuns), math.Round(avgScore*100) / 100, math.Round(bestScore*100) / 100, int(totalPassed), int(totalFailed), nil
}

type EvalService struct {
	store       EvalStore
	agentSvc    *AgentService
	agentRT     *agent.Runtime
}

func NewEvalService(store EvalStore, agentSvc *AgentService, rt *agent.Runtime) *EvalService {
	return &EvalService{store: store, agentSvc: agentSvc, agentRT: rt}
}

func (s *EvalService) ListCases(agentID int64) ([]*schema.EvalCaseResponse, error) {
	cases, err := s.store.ListCases(agentID)
	if err != nil {
		return nil, err
	}
	result := make([]*schema.EvalCaseResponse, 0, len(cases))
	for _, c := range cases {
		result = append(result, s.toCaseResponse(c))
	}
	return result, nil
}

func (s *EvalService) GetCase(id int64) (*schema.EvalCaseResponse, error) {
	c, err := s.store.GetCase(id)
	if err != nil {
		return nil, err
	}
	return s.toCaseResponse(c), nil
}

func (s *EvalService) CreateCase(req *schema.CreateEvalCaseRequest, userID int64) (*schema.EvalCaseResponse, error) {
	c := &model.EvalCase{
		Name:           req.Name,
		Description:    req.Description,
		AgentID:        req.AgentID,
		InputText:      req.InputText,
		ExpectedOutput: req.ExpectedOutput,
		Criteria:       req.Criteria,
		Tags:           req.Tags,
		IsActive:       true,
		CreatedBy:      userID,
	}
	created, err := s.store.CreateCase(c)
	if err != nil {
		return nil, err
	}
	return s.toCaseResponse(created), nil
}

func (s *EvalService) UpdateCase(id int64, req *schema.UpdateEvalCaseRequest) (*schema.EvalCaseResponse, error) {
	c, err := s.store.GetCase(id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		c.Name = *req.Name
	}
	if req.Description != nil {
		c.Description = *req.Description
	}
	if req.InputText != nil {
		c.InputText = *req.InputText
	}
	if req.ExpectedOutput != nil {
		c.ExpectedOutput = *req.ExpectedOutput
	}
	if req.Criteria != nil {
		c.Criteria = req.Criteria
	}
	if req.Tags != nil {
		c.Tags = req.Tags
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}
	updated, err := s.store.UpdateCase(id, c)
	if err != nil {
		return nil, err
	}
	return s.toCaseResponse(updated), nil
}

func (s *EvalService) DeleteCase(id int64) error {
	return s.store.DeleteCase(id)
}

func (s *EvalService) StartRun(ctx context.Context, req *schema.StartEvalRunRequest, userID int64) (*schema.EvalRunResponse, error) {
	cases, err := s.store.ListCases(req.AgentID)
	if err != nil {
		return nil, err
	}

	if len(req.CaseIDs) > 0 {
		filtered := make([]*model.EvalCase, 0, len(req.CaseIDs))
		for _, c := range cases {
			for _, rid := range req.CaseIDs {
				if c.ID == rid {
					filtered = append(filtered, c)
					break
				}
			}
		}
		cases = filtered
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no test cases found for agent")
	}

	name := req.Name
	if name == "" {
		name = fmt.Sprintf("Eval Run %s", time.Now().Format("2006-01-02 15:04"))
	}
	now := time.Now()
	run := &model.EvalRun{
		Name:      name,
		AgentID:   req.AgentID,
		Status:    "running",
		Total:     len(cases),
		StartedBy: userID,
		StartedAt: &now,
	}
	run, err = s.store.CreateRun(run)
	if err != nil {
		return nil, err
	}

	results := make([]*model.EvalResult, 0, len(cases))
	passed := 0

	for _, tc := range cases {
		startTime := time.Now()

		actual, err := s.callAgent(ctx, tc.AgentID, tc.InputText)
		duration := time.Since(startTime).Milliseconds()

		result := &model.EvalResult{
			RunID:          run.ID,
			CaseID:         tc.ID,
			CaseName:       tc.Name,
			InputText:      tc.InputText,
			ExpectedOutput: tc.ExpectedOutput,
			ActualOutput:   actual,
			DurationMs:     duration,
			Metadata:       map[string]any{},
		}

		if err != nil {
			result.Passed = false
			result.Score = 0
			result.Reason = fmt.Sprintf("Agent call failed: %v", err)
		} else {
			isPass, score, reason := s.evaluate(tc, actual)
			result.Passed = isPass
			result.Score = score
			result.Reason = reason
		}

		if result.Passed {
			passed++
		}

		s.store.CreateResult(result)
		results = append(results, result)
	}

	passedCount := passed
	failedCount := len(cases) - passed
	score := 0.0
	if len(cases) > 0 {
		score = math.Round(float64(passedCount)/float64(len(cases))*10000) / 100
	}

	endTime := time.Now()
	run.Status = "completed"
	run.Passed = passedCount
	run.Failed = failedCount
	run.Score = score
	run.EndedAt = &endTime

	summary := fmt.Sprintf("通过 %d/%d (%.1f%%)", passedCount, len(cases), score)
	run.Summary = summary
	s.store.UpdateRun(run)

	return s.toRunResponse(run, results), nil
}

func (s *EvalService) callAgent(ctx context.Context, agentID int64, input string) (string, error) {
	resp, err := s.agentRT.Chat(ctx, agentID, input, "", "eval")
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (s *EvalService) evaluate(tc *model.EvalCase, actual string) (bool, float64, string) {
	if tc.ExpectedOutput == "" {
		return true, 1.0, "无预期输出，默认通过"
	}

	pass := s.matchOutput(actual, tc.ExpectedOutput)
	if pass {
		return true, 1.0, "输出匹配预期结果"
	}

	similarity := s.computeSimilarity(actual, tc.ExpectedOutput)

	threshold := 0.6
	if t, ok := tc.Criteria["similarity_threshold"]; ok {
		if v, ok := t.(float64); ok {
			threshold = v
		}
	}

	if similarity >= threshold {
		return true, similarity, fmt.Sprintf("语义相似度 %.0f%% (阈值 %.0f%%)", similarity*100, threshold*100)
	}

	return false, similarity, fmt.Sprintf("语义相似度 %.0f%%，低于阈值 %.0f%%", similarity*100, threshold*100)
}

func (s *EvalService) matchOutput(actual, expected string) bool {
	a := strings.TrimSpace(strings.ToLower(actual))
	e := strings.TrimSpace(strings.ToLower(expected))
	return strings.Contains(a, e) || strings.Contains(e, a) || a == e
}

func (s *EvalService) computeSimilarity(a, b string) float64 {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	if a == b {
		return 1.0
	}

	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0.0
	}

	setA := make(map[string]int)
	for _, w := range wordsA {
		setA[w]++
	}
	intersection := 0
	for _, w := range wordsB {
		if setA[w] > 0 {
			intersection++
			setA[w]--
		}
	}

	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func (s *EvalService) GetRun(ctx context.Context, id int64) (*schema.EvalRunResponse, error) {
	run, err := s.store.GetRun(id)
	if err != nil {
		return nil, err
	}
	results, err := s.store.ListResultsByRun(id)
	if err != nil {
		return nil, err
	}
	return s.toRunResponse(run, results), nil
}

func (s *EvalService) ListRuns(agentID int64, limit int) ([]*schema.EvalRunResponse, error) {
	runs, err := s.store.ListRuns(agentID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]*schema.EvalRunResponse, 0, len(runs))
	for _, r := range runs {
		result = append(result, s.toRunResponse(r, nil))
	}
	return result, nil
}

func (s *EvalService) GetStats() (*schema.EvalStatsResponse, error) {
	totalCases, totalRuns, avgScore, bestScore, totalPassed, totalFailed, err := s.store.GetStats()
	if err != nil {
		return nil, err
	}

	resp := &schema.EvalStatsResponse{
		TotalCases:  totalCases,
		TotalRuns:   totalRuns,
		AvgScore:    avgScore,
		BestScore:   bestScore,
		TotalPassed: totalPassed,
		TotalFailed: totalFailed,
	}

	if totalRuns > 0 {
		runs, _ := s.store.ListRuns(0, 5)
		for _, r := range runs {
			resp.RecentRuns = append(resp.RecentRuns, s.toRunResponse(r, nil))
		}
	}

	return resp, nil
}

func (s *EvalService) toCaseResponse(c *model.EvalCase) *schema.EvalCaseResponse {
	resp := &schema.EvalCaseResponse{
		ID:             c.ID,
		Name:           c.Name,
		Description:    c.Description,
		AgentID:        c.AgentID,
		InputText:      c.InputText,
		ExpectedOutput: c.ExpectedOutput,
		Criteria:       c.Criteria,
		Tags:           c.Tags,
		IsActive:       c.IsActive,
		CreatedBy:      c.CreatedBy,
		CreatedAt:      c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.Format(time.RFC3339),
	}
	if agent, err := s.agentSvc.GetAgent(c.AgentID); err == nil && agent != nil {
		resp.AgentName = agent.Name
	}
	return resp
}

func (s *EvalService) toRunResponse(r *model.EvalRun, results []*model.EvalResult) *schema.EvalRunResponse {
	resp := &schema.EvalRunResponse{
		ID:        r.ID,
		Name:      r.Name,
		AgentID:   r.AgentID,
		Status:    r.Status,
		Total:     r.Total,
		Passed:    r.Passed,
		Failed:    r.Failed,
		Score:     r.Score,
		Summary:   r.Summary,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
	if r.StartedAt != nil {
		s := r.StartedAt.Format(time.RFC3339)
		resp.StartedAt = &s
	}
	if r.EndedAt != nil {
		s := r.EndedAt.Format(time.RFC3339)
		resp.EndedAt = &s
	}
	if agent, err := s.agentSvc.GetAgent(r.AgentID); err == nil && agent != nil {
		resp.AgentName = agent.Name
	}
	if results != nil {
		resp.Results = make([]*schema.EvalResultResp, 0, len(results))
		for _, res := range results {
			resp.Results = append(resp.Results, &schema.EvalResultResp{
				ID:             res.ID,
				RunID:          res.RunID,
				CaseID:         res.CaseID,
				CaseName:       res.CaseName,
				InputText:      res.InputText,
				ExpectedOutput: res.ExpectedOutput,
				ActualOutput:   res.ActualOutput,
				Passed:         res.Passed,
				Score:          res.Score,
				Reason:         res.Reason,
				DurationMs:     res.DurationMs,
				Metadata:       res.Metadata,
			})
		}
	}
	return resp
}

var _ EvalStore = (*evalGORMStore)(nil)
