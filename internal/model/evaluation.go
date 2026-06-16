package model

import (
	"time"
)

type EvalCase struct {
	ID             int64             `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string            `gorm:"size:100;not null" json:"name"`
	Description    string            `gorm:"size:500" json:"description"`
	AgentID        int64             `gorm:"not null;index" json:"agent_id"`
	InputText      string            `gorm:"type:text;not null" json:"input_text"`
	ExpectedOutput string            `gorm:"type:text" json:"expected_output"`
	Criteria       map[string]any    `gorm:"type:jsonb;serializer:json" json:"criteria"`
	Tags           []string          `gorm:"type:jsonb;serializer:json" json:"tags"`
	IsActive       bool              `gorm:"default:true" json:"is_active"`
	CreatedBy      int64             `json:"created_by"`
	CreatedAt      time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

func (EvalCase) TableName() string {
	return "eval_cases"
}

type EvalRun struct {
	ID        int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string        `gorm:"size:100" json:"name"`
	AgentID   int64         `gorm:"not null;index" json:"agent_id"`
	Status    string        `gorm:"size:30;default:pending" json:"status"`
	Total     int           `gorm:"default:0" json:"total"`
	Passed    int           `gorm:"default:0" json:"passed"`
	Failed    int           `gorm:"default:0" json:"failed"`
	Score     float64       `gorm:"default:0" json:"score"`
	Summary   string        `gorm:"type:text" json:"summary"`
	StartedBy int64         `json:"started_by"`
	StartedAt *time.Time    `json:"started_at,omitempty"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	CreatedAt time.Time     `gorm:"autoCreateTime" json:"created_at"`
}

func (EvalRun) TableName() string {
	return "eval_runs"
}

type EvalResult struct {
	ID             int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID          int64          `gorm:"not null;index" json:"run_id"`
	CaseID         int64          `gorm:"not null;index" json:"case_id"`
	CaseName       string         `gorm:"size:100" json:"case_name"`
	InputText      string         `gorm:"type:text" json:"input_text"`
	ExpectedOutput string         `gorm:"type:text" json:"expected_output"`
	ActualOutput   string         `gorm:"type:text" json:"actual_output"`
	Passed         bool           `json:"passed"`
	Score          float64        `gorm:"default:0" json:"score"`
	Reason         string         `gorm:"type:text" json:"reason"`
	DurationMs     int64          `json:"duration_ms"`
	Metadata       map[string]any `gorm:"type:jsonb;serializer:json" json:"metadata"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

func (EvalResult) TableName() string {
	return "eval_results"
}
