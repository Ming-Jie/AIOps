export interface UserResponse {
  id: number
  username: string
  email: string
  full_name?: string
  avatar_url?: string
  status: string
  is_admin?: boolean
  user_roles?: { id: number; role_id: number; role?: { name: string } }[]
}

export interface APIResponse<T = unknown> {
  code: number
  message: string
  data?: T
}

export interface Skill {
  id: number
  key: string
  name: string
  description?: string
  content?: string
  source_ref: string
  prompt_hint?: string
  category?: string
  risk_level?: string
  execution_mode?: string
  is_active: boolean
  created_by: number
  created_at: string
  updated_at?: string
}

export interface MCPTool {
  tool_name: string
  display_name?: string
  description?: string
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  is_active?: boolean
}

export interface NotifyChannel {
  id: number
  name: string
  kind: 'lark' | 'dingtalk' | 'wecom' | string
  webhook_url?: string
  app_id?: string
  has_app_secret: boolean
  extra?: Record<string, string>
  is_active: boolean
  created_at: string
}

export interface MCPConfig {
  id: number
  key: string
  name: string
  description?: string
  transport: string
  endpoint?: string
  config?: Record<string, unknown>
  config_json?: string
  is_active: boolean
  health_status: string
  tool_count: number
  created_by: number
  validation_status?: string
  tools?: MCPTool[]
  created_at: string
  updated_at?: string
}

export interface Agent {
  id: number
  public_id: string
  name: string
  description: string
  category: string
  is_builtin?: boolean
  is_active?: boolean
  created_at?: string
  updated_at?: string
  skill_ids?: string[]
  mcp_config_ids?: number[]
  kb_ids?: number[]
  chat_user_ids?: number[]
  can_chat?: boolean
  can_edit?: boolean
}

export interface RuntimeProfile {
  source_agent: string
  archetype: string
  role?: string
  goal?: string
  backstory?: string
  system_prompt?: string
  llm_model?: string
  model_config_id?: number
  temperature: number
  stream_enabled: boolean
  memory_enabled: boolean
  skill_ids?: string[]
  mcp_config_ids?: number[]
  kb_ids?: number[]
  execution_mode?: string
  max_iterations?: number
  plan_prompt?: string
  reflection_depth?: number
  approval_mode?: string
}

export interface CreateAgentRequest {
  name: string
  description: string
  category: string
  is_active?: boolean
  runtime_profile?: RuntimeProfile
  chat_user_ids?: number[]
}

export interface UpdateAgentRequest {
  name?: string
  description?: string
  category?: string
  is_active?: boolean
  runtime_profile?: RuntimeProfile
  chat_user_ids?: number[]
}

export interface ChatSession {
  session_id: string
  agent_id: number
  user_id?: string
  title?: string
  created_at: string
  updated_at: string
  im_channel?: string
  im_user_id?: string
}

export interface ChatReactStep {
  type: string
  data: Record<string, unknown>
  meta?: Record<string, unknown>
  timestamp?: string
}

export interface FileAttachment {
  filename: string
  mime_type?: string
  size?: number
  inline?: string
  url?: string
}

export interface ChatHistoryMessage {
  id: number
  agent_id?: number
  role: string
  content: string
  image_urls: string[]
  file_urls: string[]
  react_steps?: unknown[]
  created_at: string
}

export interface CreateSkillRequest {
  key: string
  name: string
  description?: string
  content?: string
  source_ref: string
}

export type Action = 1 | 2 | 4 | 8

export interface Permission {
  id: number
  resource_type: string
  resource_name: string
  actions: Action
  description?: string
  is_system: boolean
  created_at: string
}

export interface Role {
  id: number
  name: string
  description?: string
  is_system: boolean
  is_active: boolean
  permissions?: Permission[]
  user_count?: number
  agent_count?: number
  created_at: string
  updated_at: string
}

export interface UserRole {
  id: number
  user_id: number
  role_id: number
  is_active: boolean
  expires_at?: string
  created_at: string
  role?: Role
}

export interface MessageChannel {
  id: number
  name: string
  agent_id: number
  agent_name?: string
  kind: 'direct' | 'broadcast' | 'topic' | string
  description: string
  is_public: boolean
  metadata?: Record<string, string>
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface AgentMessage {
  id: number
  from_agent_id: number
  from_agent_name?: string
  to_agent_id: number
  to_agent_name?: string
  channel_id: number
  session_id: string
  kind: 'text' | 'command' | 'event' | 'result' | string
  content: string
  metadata?: Record<string, unknown>
  status: string
  priority: number
  created_at: string
  delivered_at?: string
}

export interface A2ACard {
  id: number
  agent_id: number
  agent_name?: string
  name: string
  description: string
  url: string
  version: string
  capabilities?: string[]
  is_active: boolean
  created_at: string
}

export interface WorkflowNode {
  id: string
  type: string
  label: string
  agent_id?: number
  config?: Record<string, unknown>
  position?: { x: number; y: number }
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
}

export interface WorkflowEdge {
  id: string
  source_node_id: string
  source_port?: string
  target_node_id: string
  target_port?: string
  condition?: string
  label?: string
}

export interface WorkflowDefinition {
  id: number
  key: string
  name: string
  description?: string
  kind: string
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
  variables?: Record<string, unknown>
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  version: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateWorkflowDefinitionRequest {
  key: string
  name: string
  description?: string
  kind: string
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
  variables?: Record<string, unknown>
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  is_active?: boolean
}

export interface UpdateWorkflowDefinitionRequest {
  name?: string
  description?: string
  kind?: string
  nodes?: WorkflowNode[]
  edges?: WorkflowEdge[]
  variables?: Record<string, unknown>
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  is_active?: boolean
}

export interface ExecuteWorkflowResponse {
  output: unknown
  node_results?: Record<string, unknown>
  node_result_order?: string[]
  duration_ms: number
  execution_id?: number
}

export interface WorkflowExecution {
  id: number
  workflow_id: number
  workflow_key: string
  status: string
  input: string
  output: string
  error: string
  node_results: Record<string, unknown>[]
  variables: Record<string, unknown>
  duration_ms: number
  started_at: string
  finished_at: string
  created_by: string
}

export interface Schedule {
  id: number
  name: string
  description?: string
  agent_id?: number
  workflow_id?: number
  workflow_name?: string
  agent_name?: string
  code_language?: string
  channel_id?: number
  channel_name?: string
  schedule_kind: 'at' | 'every' | 'cron'
  cron_expr?: string
  at?: string
  every_ms?: number
  timezone?: string
  wake_mode: string
  session_target: string
  chat_session_id?: string
  prompt: string
  stagger_ms: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateScheduleRequest {
  name: string
  description?: string
  agent_id?: number
  workflow_id?: number
  channel_id?: number
  code_language?: string
  schedule_kind: 'at' | 'every' | 'cron'
  cron_expr?: string
  at?: string
  every_ms?: number
  timezone?: string
  wake_mode?: string
  session_target?: string
  prompt: string
  stagger_ms?: number
  enabled?: boolean
}

export interface UpdateScheduleRequest {
  name?: string
  description?: string
  agent_id?: number
  workflow_id?: number
  channel_id?: number
  code_language?: string
  schedule_kind?: string
  cron_expr?: string
  at?: string
  every_ms?: number
  timezone?: string
  wake_mode?: string
  session_target?: string
  prompt?: string
  stagger_ms?: number
  enabled?: boolean
}

export interface ModelConfig {
  id: number
  name: string
  provider: string
  model: string
  base_url?: string
  api_key?: string
  config?: Record<string, unknown>
  is_active: boolean
  purpose: string
  created_at: string
  updated_at: string
}

export interface KnowledgeBase {
  id: number
  owner_id: number
  name: string
  description: string
  visibility: string
  viking_path: string
  doc_count: number
  is_owner: boolean
  can_manage: boolean
  created_at: string
  updated_at: string
}

export interface KBDocument {
  id: number
  kb_id: number
  owner_id: number
  filename: string
  viking_uri: string
  size: number
  status: string
  error?: string
  task_id?: string
  created_at: string
  updated_at: string
}

export interface KBImportURLFailure {
  url: string
  message: string
}

export interface KBImportURLsResult {
  imported: KBDocument[]
  failed: KBImportURLFailure[]
}

export interface KBSearchHit {
  uri: string
  abstract: string
  overview?: string
  content?: string
  score: number
  context_type: string
  level: number
}
