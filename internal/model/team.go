package model

import (
	"time"
)

type TeamCollaborationMode string

const (
	TeamModeGroupChat   TeamCollaborationMode = "group_chat"
	TeamModeDebate      TeamCollaborationMode = "debate"
	TeamModeRouting     TeamCollaborationMode = "routing"
	TeamModeSequential  TeamCollaborationMode = "sequential"
)

type Team struct {
	ID          int64                  `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string                 `gorm:"size:100;not null" json:"name"`
	Description string                 `gorm:"size:500" json:"description"`
	Mode        TeamCollaborationMode  `gorm:"size:30;not null;default:group_chat" json:"mode"`
	CoordinatorAgentID *int64          `json:"coordinator_agent_id,omitempty"`
	MaxRounds   int                    `gorm:"default:5" json:"max_rounds"`
	IsActive    bool                   `gorm:"default:true" json:"is_active"`
	Config      map[string]any         `gorm:"type:jsonb;serializer:json" json:"config"`
	CreatedBy   int64                  `json:"created_by"`
	CreatedAt   time.Time              `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time              `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Team) TableName() string {
	return "teams"
}

type TeamMember struct {
	ID            int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	TeamID        int64  `gorm:"not null;index;uniqueIndex:idx_team_agent" json:"team_id"`
	AgentID       int64  `gorm:"not null;index;uniqueIndex:idx_team_agent" json:"agent_id"`
	Role          string `gorm:"size:50;default:member" json:"role"`
	SortOrder     int    `gorm:"default:0" json:"sort_order"`
	IsActive      bool   `gorm:"default:true" json:"is_active"`
}

func (TeamMember) TableName() string {
	return "team_members"
}

type TeamConversation struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TeamID    int64     `gorm:"not null;index" json:"team_id"`
	Title     string    `gorm:"size:200" json:"title"`
	Status    string    `gorm:"size:30;default:active" json:"status"`
	StartedBy int64     `json:"started_by"`
	Round     int       `gorm:"default:0" json:"round"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TeamConversation) TableName() string {
	return "team_conversations"
}

type TeamMessage struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID int64     `gorm:"not null;index" json:"conversation_id"`
	TeamID         int64     `gorm:"not null;index" json:"team_id"`
	SenderAgentID  int64     `json:"sender_agent_id"`
	SenderName     string    `gorm:"size:100" json:"sender_name"`
	Content        string    `gorm:"type:text" json:"content"`
	MsgType        string    `gorm:"size:30;default:message" json:"msg_type"`
	TargetAgentID  *int64    `json:"target_agent_id,omitempty"`
	Round          int       `gorm:"default:0" json:"round"`
	Metadata       map[string]any `gorm:"type:jsonb;serializer:json" json:"metadata"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TeamMessage) TableName() string {
	return "team_messages"
}
