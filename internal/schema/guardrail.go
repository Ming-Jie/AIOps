package schema

type CreateGuardrailRuleRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description,omitempty"`
	RuleType    string `json:"rule_type" validate:"required"`
	Scope       string `json:"scope,omitempty"`
	Action      string `json:"action,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Priority    *int   `json:"priority,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	AgentIDs    []int64 `json:"agent_ids,omitempty"`
}

type UpdateGuardrailRuleRequest struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	RuleType    *string        `json:"rule_type,omitempty"`
	Scope       *string        `json:"scope,omitempty"`
	Action      *string        `json:"action,omitempty"`
	Severity    *string        `json:"severity,omitempty"`
	Priority    *int           `json:"priority,omitempty"`
	IsActive    *bool          `json:"is_active,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	AgentIDs    []int64        `json:"agent_ids,omitempty"`
}

type GuardrailRuleResponse struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	RuleType    string           `json:"rule_type"`
	Scope       string           `json:"scope"`
	Action      string           `json:"action"`
	Severity    string           `json:"severity"`
	Priority    int              `json:"priority"`
	IsActive    bool             `json:"is_active"`
	Config      map[string]any   `json:"config"`
	HitCount    int64            `json:"hit_count"`
	LastHitAt   *string          `json:"last_hit_at,omitempty"`
	CreatedBy   int64            `json:"created_by"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	BoundAgents []*AgentBrief    `json:"bound_agents,omitempty"`
}

type GuardrailLogResponse struct {
	ID        int64          `json:"id"`
	RuleID    *int64         `json:"rule_id,omitempty"`
	RuleName  string         `json:"rule_name"`
	RuleType  string         `json:"rule_type"`
	AgentID   int64          `json:"agent_id"`
	Scope     string         `json:"scope"`
	Action    string         `json:"action"`
	Severity  string         `json:"severity"`
	UserID    int64          `json:"user_id"`
	SessionID string         `json:"session_id"`
	Input     string         `json:"input"`
	Output    string         `json:"output"`
	MatchInfo map[string]any `json:"match_info"`
	Blocked   bool           `json:"blocked"`
	CreatedAt string         `json:"created_at"`
}

type ListGuardrailLogsRequest struct {
	RuleType string `json:"rule_type,omitempty"`
	AgentID  int64  `json:"agent_id,omitempty"`
	Blocked  *bool  `json:"blocked,omitempty"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

type TestGuardrailRequest struct {
	RuleID int64  `json:"rule_id,omitempty"`
	Text   string `json:"text" validate:"required"`
	Scope  string `json:"scope,omitempty"`
}

type TestGuardrailResponse struct {
	Triggered bool           `json:"triggered"`
	RuleName  string         `json:"rule_name,omitempty"`
	RuleType  string         `json:"rule_type,omitempty"`
	Action    string         `json:"action,omitempty"`
	MatchInfo map[string]any `json:"match_info,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}
