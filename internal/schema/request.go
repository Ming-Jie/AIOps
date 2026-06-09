package schema

import "time"

type CreateAgentRequest struct {
	Name           string          `json:"name" validate:"required,min=1,max=100"`
	Description    string          `json:"description" validate:"required,min=1,max=1000"`
	Category       string          `json:"category"`
	RuntimeProfile *RuntimeProfile `json:"runtime_profile,omitempty"`
}

type UpdateAgentRequest struct {
	Name           string          `json:"name,omitempty"`
	Description    string          `json:"description,omitempty"`
	Category       string          `json:"category,omitempty"`
	RuntimeProfile *RuntimeProfile `json:"runtime_profile,omitempty"`
}

type UpdateCapabilityTreeRequest struct {
	Nodes []CapabilityTreeNode `json:"nodes"`
}

type CreateSkillRequest struct {
	Key         string `json:"key" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	Content     string `json:"content"`
	SourceRef   string `json:"source_ref" validate:"required"`
}

type CreateLearningRequest struct {
	UserID    *int64 `json:"user_id,omitempty"` // nil = global
	ErrorType string `json:"error_type" validate:"required"`
	Context   string `json:"context"`
	RootCause string `json:"root_cause"`
	Fix       string `json:"fix"`
	Lesson    string `json:"lesson"`
}

// UpdateSkillRequest partial update for PUT /skills/:id (nil pointer = omit field).
type UpdateSkillRequest struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	Content       *string `json:"content,omitempty"`
	SourceRef     *string `json:"source_ref,omitempty"`
	RiskLevel     *string `json:"risk_level,omitempty"`
	ExecutionMode *string `json:"execution_mode,omitempty"`
	PromptHint    *string `json:"prompt_hint,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
}

type CreateMCPConfigRequest struct {
	Key       string         `json:"key" validate:"required"`
	Name      string         `json:"name" validate:"required"`
	Transport string         `json:"transport"`
	Endpoint  string         `json:"endpoint"`
	Config    map[string]any `json:"config,omitempty"`
}

type SyncMCPServerRequest struct {
	Tools            []MCPServer `json:"tools"`
	CreateCapability bool        `json:"create_capability"`
}

// ChatRequest body for POST /chat (JSON reply) and POST /chat/stream (SSE).
// Set either agent_id (single agent) or workflow_id (multi-agent orchestration), not both.
// SessionID is optional: omit for stateless chat; for multi-turn memory, call POST /chat/sessions first and send the returned session_id here.
// ChatImagePart is one vision image in a multi-image user message (POST /chat/stream body).
type ChatImagePart struct {
	Base64 string `json:"base64"`
	Mime   string `json:"mime"`
}

type ChatRequest struct {
	AgentID     int64           `json:"agent_id"`
	WorkflowID  int64           `json:"workflow_id,omitempty"`
	Message     string          `json:"message"`
	SessionID   string          `json:"session_id,omitempty"`
	ClientType  string          `json:"client_type,omitempty"`
	ImageBase64 string          `json:"image_base64,omitempty"`
	ImageMime   string          `json:"image_mime,omitempty"`
	ImageParts  []ChatImagePart `json:"image_parts,omitempty"`
	ImageURL    string          `json:"image_url,omitempty"`
	ImageURLs   []string        `json:"image_urls,omitempty"`
	FileURLs    []string        `json:"file_urls,omitempty"`
}

// ChatSession is a persisted conversation scope (listable, queryable history).
type ChatSession struct {
	SessionID string    `json:"session_id"`
	AgentID   int64     `json:"agent_id"`
	UserID    string    `json:"user_id,omitempty"`
	Title     string    `json:"title,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// IM bot sessions (user_id im:lark:… / im:telegram:…)
	IMChannel string `json:"im_channel,omitempty"`
	IMUserID  string `json:"im_user_id,omitempty"`
}

// CreateChatSessionRequest body for POST /chat/sessions (user comes from JWT, not body).
type CreateChatSessionRequest struct {
	AgentID int64 `json:"agent_id" validate:"required,min=1"`
}

// UpdateChatSessionRequest body for PUT /chat/sessions/:session_id.
type UpdateChatSessionRequest struct {
	Title string `json:"title"`
}

// StopChatRequest body for POST /chat/stop
type StopChatRequest struct {
	SessionID string `json:"session_id" validate:"required"`
}

// ChatHistoryMessage is one stored turn in a session (no embedding in API).
// Use image_urls (and file_urls); legacy single image_url in extra JSON is merged into image_urls when listing.
// image_urls and file_urls are always emitted (may be empty) so clients can rely on keys for history UI.
type ChatHistoryMessage struct {
	ID        int64    `json:"id"`
	AgentID   int64    `json:"agent_id,omitempty"`
	Role      string   `json:"role"`
	Content   string   `json:"content"`
	ImageURLs []string `json:"image_urls"`
	FileURLs  []string `json:"file_urls"`
	// ReactSteps is persisted from SSE (ReAct / ADK) for chat UI replay after refresh.
	ReactSteps []map[string]any `json:"react_steps,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

type ChatResponse struct {
	Message    string `json:"message"`
	SessionID  string `json:"session_id"`
	AgentID    int64  `json:"agent_id,omitempty"`
	WorkflowID int64  `json:"workflow_id,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func SuccessResponse(data any) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func ErrorResponse(message string) APIResponse {
	return APIResponse{Code: 1, Message: message}
}

type UploadResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

const (
	ExecutionModeServer = "server"
	ExecutionModeClient = "client"
)

type ClientToolCall struct {
	CallID        string         `json:"call_id"`
	ToolName      string         `json:"tool_name"`
	Params        map[string]any `json:"params"`
	Hint          string         `json:"hint,omitempty"`
	RiskLevel     string         `json:"risk_level,omitempty"`
	ExecutionMode string         `json:"execution_mode,omitempty"` // client | server; desktop confirms only client + risk > low
}

type SubmitToolResultRequest struct {
	SessionID string `json:"session_id" validate:"required"`
	CallID    string `json:"call_id" validate:"required"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	// ClientType: same as ChatStreamRequest — "desktop" for Tauri; omit or "web" for browser. Normalized with User-Agent in controller.
	ClientType string `json:"client_type,omitempty"`
}

type ChatResponseWithToolCall struct {
	Message        string          `json:"message"`
	SessionID      string          `json:"session_id"`
	AgentID        int64           `json:"agent_id,omitempty"`
	WorkflowID     int64           `json:"workflow_id,omitempty"`
	DurationMS     int64           `json:"duration_ms,omitempty"`
	ClientToolCall *ClientToolCall `json:"client_tool_call,omitempty"`
}
