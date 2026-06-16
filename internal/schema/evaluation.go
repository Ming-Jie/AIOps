package schema

type CreateEvalCaseRequest struct {
	Name           string         `json:"name" validate:"required,min=1,max=100"`
	Description    string         `json:"description,omitempty"`
	AgentID        int64          `json:"agent_id" validate:"required"`
	InputText      string         `json:"input_text" validate:"required"`
	ExpectedOutput string         `json:"expected_output,omitempty"`
	Criteria       map[string]any `json:"criteria,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
}

type UpdateEvalCaseRequest struct {
	Name           *string        `json:"name,omitempty"`
	Description    *string        `json:"description,omitempty"`
	InputText      *string        `json:"input_text,omitempty"`
	ExpectedOutput *string        `json:"expected_output,omitempty"`
	Criteria       map[string]any `json:"criteria,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	IsActive       *bool          `json:"is_active,omitempty"`
}

type EvalCaseResponse struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	AgentID        int64          `json:"agent_id"`
	AgentName      string         `json:"agent_name,omitempty"`
	InputText      string         `json:"input_text"`
	ExpectedOutput string         `json:"expected_output"`
	Criteria       map[string]any `json:"criteria"`
	Tags           []string       `json:"tags"`
	IsActive       bool           `json:"is_active"`
	CreatedBy      int64          `json:"created_by"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

type StartEvalRunRequest struct {
	Name    string  `json:"name,omitempty"`
	AgentID int64   `json:"agent_id" validate:"required"`
	CaseIDs []int64 `json:"case_ids,omitempty"`
}

type EvalRunResponse struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	AgentID   int64              `json:"agent_id"`
	AgentName string             `json:"agent_name,omitempty"`
	Status    string             `json:"status"`
	Total     int                `json:"total"`
	Passed    int                `json:"passed"`
	Failed    int                `json:"failed"`
	Score     float64            `json:"score"`
	Summary   string             `json:"summary"`
	Results   []*EvalResultResp  `json:"results,omitempty"`
	StartedAt *string            `json:"started_at,omitempty"`
	EndedAt   *string            `json:"ended_at,omitempty"`
	CreatedAt string             `json:"created_at"`
}

type EvalResultResp struct {
	ID             int64          `json:"id"`
	RunID          int64          `json:"run_id"`
	CaseID         int64          `json:"case_id"`
	CaseName       string         `json:"case_name"`
	InputText      string         `json:"input_text"`
	ExpectedOutput string         `json:"expected_output"`
	ActualOutput   string         `json:"actual_output"`
	Passed         bool           `json:"passed"`
	Score          float64        `json:"score"`
	Reason         string         `json:"reason"`
	DurationMs     int64          `json:"duration_ms"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type EvalStatsResponse struct {
	TotalCases      int     `json:"total_cases"`
	TotalRuns       int     `json:"total_runs"`
	AvgScore        float64 `json:"avg_score"`
	BestScore       float64 `json:"best_score"`
	TotalPassed     int     `json:"total_passed"`
	TotalFailed     int     `json:"total_failed"`
	RecentRuns      []*EvalRunResponse `json:"recent_runs,omitempty"`
}
