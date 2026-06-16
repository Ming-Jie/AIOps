package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/components/model"
	"github.com/fisk086/aiops/internal/approval"
	"github.com/fisk086/aiops/internal/mcp"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/skills"
	storepkg "github.com/fisk086/aiops/internal/storage"
)



type Runtime struct {
	mu         sync.RWMutex
	agents     map[int64]*schema.AgentWithRuntime
	byName     map[string]*schema.AgentWithRuntime
	byCategory map[string][]*schema.AgentWithRuntime

	chatModel      model.ToolCallingChatModel
	modelProvider  ModelProvider
	mcpClient      *mcp.Client
	skillLoader      *skills.Loader
	skillRegistry    *skills.Registry
	store            any
	auditLogger      *AuditLogger
	clientToolMgr    *ClientToolManager
	approvalProvider approval.ExternalApprovalProvider
	approvalNotifier approval.ApprovalNotifier

	stopStreams map[string]chan struct{}

	usageSink TokenUsageSink

	systemPromptCache sync.Map // map[int64]string — agentID → base system prompt

	// defaultChatModelName used for token usage records when agent has no per-agent llm_model.
	defaultChatModelName string

	// Optional RAG context provider (IM / scheduler paths that bypass ChatService).
	kbContextProvider func(ctx context.Context, agentID int64, userText string) string

	// Optional code executor (sandbox: Docker / k8s / host). Used by builtin_run_python tool.
	codeExecutor func(ctx context.Context, language, code string, input map[string]any) (string, error)
}

func NewRuntime() *Runtime {
	return &Runtime{
		agents:      make(map[int64]*schema.AgentWithRuntime),
		byName:      make(map[string]*schema.AgentWithRuntime),
		byCategory:  make(map[string][]*schema.AgentWithRuntime),
		stopStreams: make(map[string]chan struct{}),
	}
}

func NewRuntimeWithSkill(chatModel model.ToolCallingChatModel, mcpClient *mcp.Client, skillLoader *skills.Loader, skillRegistry *skills.Registry, store any) *Runtime {
	r := NewRuntime()
	r.chatModel = chatModel
	r.mcpClient = mcpClient
	r.skillLoader = skillLoader
	r.skillRegistry = skillRegistry
	r.store = store
	r.clientToolMgr = NewClientToolManager()
	if as, ok := store.(AuditStore); ok {
		r.auditLogger = NewAuditLogger(as)
	}
	return r
}

func (r *Runtime) SetModelProvider(p ModelProvider) {
	r.modelProvider = p
	r.chatModel = &resolvableModel{
		defaultModel: r.chatModel,
		provider:     p,
		agents:       r.GetAgent,
	}
}

func (r *Runtime) SetKBContextProvider(fn func(ctx context.Context, agentID int64, userText string) string) {
	r.kbContextProvider = fn
}

// cachedSystemPrompt returns the base system prompt (Role/Goal/Backstory/SystemPrompt) for an agent,
// building and caching it once per agent registration. Cache is invalidated on RegisterAgent/UnregisterAgent.
func (r *Runtime) cachedSystemPrompt(agentID int64, profile *schema.RuntimeProfile) string {
	if profile == nil {
		return ""
	}
	if cached, ok := r.systemPromptCache.Load(agentID); ok {
		return cached.(string)
	}
	p := profile.SystemPrompt
	if p == "" {
		p = buildSystemPrompt(profile)
	}
	r.systemPromptCache.Store(agentID, p)
	return p
}

func (r *Runtime) SetCodeExecutor(fn func(ctx context.Context, language, code string, input map[string]any) (string, error)) {
	r.codeExecutor = fn
}

// SummarizeMessages uses the chat model to produce a concise summary of the given messages.
func (r *Runtime) SummarizeMessages(ctx context.Context, msgs []*einoschema.Message) (string, error) {
	if r.chatModel == nil {
		return "", fmt.Errorf("chat model not configured")
	}
	prompt := "请用中文简要总结以下对话内容，保留关键信息和要点："
	summarizeMsgs := []*einoschema.Message{
		{Role: einoschema.System, Content: "你是一个专业的对话摘要助手。请用简洁的语言总结对话的核心内容，包括主要话题、关键决定和重要结论。"},
		{Role: einoschema.User, Content: prompt + "\n\n" + joinMessagesForSummary(msgs)},
	}
	resp, err := r.chatModel.Generate(ctx, summarizeMsgs)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func joinMessagesForSummary(msgs []*einoschema.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case einoschema.User:
			b.WriteString("用户: ")
		case einoschema.Assistant:
			b.WriteString("助理: ")
		case einoschema.Tool:
			b.WriteString("工具: ")
		default:
			b.WriteString("系统: ")
		}
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func (r *Runtime) mergeKBContext(ctx context.Context, agentID int64, userText, memContext string) string {
	if r.kbContextProvider == nil {
		return memContext
	}
	kbContext := r.kbContextProvider(ctx, agentID, userText)
	if kbContext == "" {
		return memContext
	}
	if memContext != "" {
		return kbContext + "\n\n" + memContext
	}
	return kbContext
}

func (r *Runtime) AuditLogger() *AuditLogger {
	return r.auditLogger
}

func (r *Runtime) GetStatePlanMode(callID string) (bool, error) {
	return r.clientToolMgr.GetStatePlanMode(callID)
}

func (r *Runtime) resolveToolRiskLevel(toolName string) string {
	if store, ok := r.store.(storepkg.Storage); ok {
		if skills, err := store.ListSkills(); err == nil {
			skillKey := normalizeToolToSkillKey(toolName)
			for _, sk := range skills {
				if sk == nil || sk.Key != skillKey {
					continue
				}
				if s := strings.TrimSpace(sk.RiskLevel); s != "" {
					return strings.ToLower(s)
				}
				break
			}
		}
	}
	return getToolRiskLevel(toolName)
}

func (r *Runtime) RegisterAgent(agent *schema.AgentWithRuntime) {
	if agent == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agent.ID] = agent
	r.byName[agent.Name] = agent
	r.byCategory[agent.Category] = append(r.byCategory[agent.Category], agent)
	r.systemPromptCache.Delete(agent.ID)
}

func (r *Runtime) UnregisterAgent(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[id]
	if !ok {
		return
	}
	delete(r.agents, id)
	delete(r.byName, agent.Name)
	r.systemPromptCache.Delete(id)
}

func (r *Runtime) GetAgent(id int64) (*schema.AgentWithRuntime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[id]
	return agent, ok
}

// agentWithSkillOverrides returns a shallow copy of the registered agent with SkillExecutionOverrides filled from DB
// (skills.execution_mode wins over SKILL/code defaults when both differ). UI changes apply on the next request without restart.
func (r *Runtime) agentWithSkillOverrides(src *schema.AgentWithRuntime) *schema.AgentWithRuntime {
	if src == nil {
		return nil
	}
	out := *src
	if st, ok := r.store.(storepkg.Storage); ok {
		out.SkillExecutionOverrides = SkillExecutionOverridesFromStore(st, &out)
	}
	return &out
}

func (r *Runtime) GetAgentByName(name string) (*schema.AgentWithRuntime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.byName[name]
	return agent, ok
}

func (r *Runtime) ListAgents() []*schema.AgentWithRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agents := make([]*schema.AgentWithRuntime, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (r *Runtime) ListAgentsByCategory(category string) []*schema.AgentWithRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byCategory[category]
}

func (r *Runtime) RegisterStreamStop(sessionID string) chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan struct{})
	r.stopStreams[sessionID] = ch
	return ch
}

func (r *Runtime) StopStream(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.stopStreams[sessionID]; ok {
		close(ch)
		delete(r.stopStreams, sessionID)
	}
}


