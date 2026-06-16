package schema

type CreateTeamRequest struct {
	Name               string `json:"name" validate:"required,min=1,max=100"`
	Description        string `json:"description,omitempty"`
	Mode               string `json:"mode,omitempty"`
	CoordinatorAgentID *int64 `json:"coordinator_agent_id,omitempty"`
	MaxRounds          *int   `json:"max_rounds,omitempty"`
	IsActive           *bool  `json:"is_active,omitempty"`
	Config             map[string]any `json:"config,omitempty"`
	AgentIDs           []int64 `json:"agent_ids,omitempty"`
}

type UpdateTeamRequest struct {
	Name               *string        `json:"name,omitempty"`
	Description        *string        `json:"description,omitempty"`
	Mode               *string        `json:"mode,omitempty"`
	CoordinatorAgentID *int64         `json:"coordinator_agent_id,omitempty"`
	MaxRounds          *int           `json:"max_rounds,omitempty"`
	IsActive           *bool          `json:"is_active,omitempty"`
	Config             map[string]any `json:"config,omitempty"`
	AgentIDs           []int64        `json:"agent_ids,omitempty"`
}

type TeamResponse struct {
	ID                 int64            `json:"id"`
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	Mode               string           `json:"mode"`
	CoordinatorAgentID *int64           `json:"coordinator_agent_id,omitempty"`
	MaxRounds          int              `json:"max_rounds"`
	IsActive           bool             `json:"is_active"`
	Config             map[string]any   `json:"config"`
	CreatedBy          int64            `json:"created_by"`
	CreatedAt          string           `json:"created_at"`
	UpdatedAt          string           `json:"updated_at"`
	Members            []*TeamMemberResp `json:"members,omitempty"`
}

type TeamMemberResp struct {
	ID        int64  `json:"id"`
	TeamID    int64  `json:"team_id"`
	AgentID   int64  `json:"agent_id"`
	AgentName string `json:"agent_name,omitempty"`
	Role      string `json:"role"`
	SortOrder int    `json:"sort_order"`
}

type TeamConversationResponse struct {
	ID        int64             `json:"id"`
	TeamID    int64             `json:"team_id"`
	Title     string            `json:"title"`
	Status    string            `json:"status"`
	StartedBy int64             `json:"started_by"`
	Round     int               `json:"round"`
	Messages  []*TeamMsgResp    `json:"messages,omitempty"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

type TeamMsgResp struct {
	ID            int64          `json:"id"`
	ConversationID int64         `json:"conversation_id"`
	SenderAgentID int64          `json:"sender_agent_id"`
	SenderName    string         `json:"sender_name"`
	Content       string         `json:"content"`
	MsgType       string         `json:"msg_type"`
	TargetAgentID *int64         `json:"target_agent_id,omitempty"`
	Round         int            `json:"round"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     string         `json:"created_at"`
}

type SendTeamMessageRequest struct {
	ConversationID int64  `json:"conversation_id"`
	Text           string `json:"text" validate:"required"`
}

type AgentBrief struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
