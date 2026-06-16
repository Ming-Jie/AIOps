package model

import (
	"time"
)

type GuardrailAction string

const (
	GuardrailActionBlock    GuardrailAction = "block"
	GuardrailActionMask     GuardrailAction = "mask"
	GuardrailActionWarn     GuardrailAction = "warn"
	GuardrailActionAllow    GuardrailAction = "allow"
	GuardrailActionRedirect GuardrailAction = "redirect"
)

type GuardrailRuleScope string

const (
	GuardrailScopeInput  GuardrailRuleScope = "input"
	GuardrailScopeOutput GuardrailRuleScope = "output"
	GuardrailScopeBoth   GuardrailRuleScope = "both"
)

type GuardrailRule struct {
	ID          int64             `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string            `gorm:"size:100;not null" json:"name"`
	Description string            `gorm:"size:500" json:"description"`
	RuleType    string            `gorm:"size:50;not null;index" json:"rule_type"`
	Scope       GuardrailRuleScope `gorm:"size:20;default:both" json:"scope"`
	Action      GuardrailAction   `gorm:"size:20;default:block" json:"action"`
	Severity    string            `gorm:"size:20;default:medium" json:"severity"`
	Priority    int               `gorm:"default:0" json:"priority"`
	IsActive    bool              `gorm:"default:true" json:"is_active"`

	Config map[string]any `gorm:"type:jsonb;serializer:json" json:"config"`

	HitCount    int64     `gorm:"default:0" json:"hit_count"`
	LastHitAt   *time.Time `json:"last_hit_at,omitempty"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (GuardrailRule) TableName() string {
	return "guardrail_rules"
}

type GuardrailAgentBinding struct {
	ID       int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleID   int64 `gorm:"not null;index;uniqueIndex:idx_rule_agent" json:"rule_id"`
	AgentID  int64 `gorm:"not null;index;uniqueIndex:idx_rule_agent" json:"agent_id"`
	IsActive bool  `gorm:"default:true" json:"is_active"`
}

func (GuardrailAgentBinding) TableName() string {
	return "guardrail_agent_bindings"
}

type GuardrailLog struct {
	ID        int64            `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleID    *int64           `gorm:"index" json:"rule_id,omitempty"`
	RuleName  string           `gorm:"size:100" json:"rule_name"`
	RuleType  string           `gorm:"size:50;index" json:"rule_type"`
	AgentID   int64            `gorm:"index" json:"agent_id"`
	Scope     string           `gorm:"size:20" json:"scope"`
	Action    GuardrailAction  `gorm:"size:20" json:"action"`
	Severity  string           `gorm:"size:20" json:"severity"`
	UserID    int64            `json:"user_id"`
	SessionID string           `gorm:"size:100" json:"session_id"`
	Input     string           `gorm:"type:text" json:"input"`
	Output    string           `gorm:"type:text" json:"output"`
	MatchInfo map[string]any   `gorm:"type:jsonb;serializer:json" json:"match_info"`
	Blocked   bool             `gorm:"default:false" json:"blocked"`
	CreatedAt time.Time        `gorm:"autoCreateTime" json:"created_at"`
}

func (GuardrailLog) TableName() string {
	return "guardrail_logs"
}
