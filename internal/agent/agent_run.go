package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/approval"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	agentmodel "github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/skills"
)

func (r *Runtime) ResolveCapabilities(agentID int64) []schema.Capability {
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return nil
	}
	return agent.Capabilities
}

func (r *Runtime) runAgent(ctx context.Context, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, sessionID, auditUserID string) (string, *ReActResult, error) {
	ctx = r.ensureUsageTracking(ctx, agent, auditUserID)
	defer r.flushUsageSession(ctx)

	if agent.RuntimeProfile != nil && len(agent.RuntimeProfile.KBIDs) > 0 {
		logger.Info("knowledge bases bound for agent",
			"agent_id", agent.ID,
			"kb_ids", agent.RuntimeProfile.KBIDs,
			"kb_retrieval_enabled", r.kbContextProvider != nil,
		)
	}

	tools, err := r.allToolsForAgent(agent)
	if err != nil {
		return "", nil, err
	}
	if _, ok := imoutbound.ScopeFromContext(ctx); ok {
		tools = append(tools, skills.NewIMOutboundFileTool(imoutbound.GlobalStore()))
		logger.Info("runAgent: IM outbound file tool bound", "agent_id", agent.ID, "session_id", sessionID)
	} else {
		tools = append(tools, skills.NewWebSaveFileTool())
	}
	tools = r.wrapToolsWithAudit(agent, sessionID, auditUserID, tools)

	execMode := ExecutionModeDefault
	if agent.RuntimeProfile != nil {
		execMode = agent.RuntimeProfile.ExecutionMode
	}

	if execMode == ExecutionModeAuto {
		mode, err := r.resolveExecutionModeAuto(ctx, agent, userInput, history, tools)
		if err != nil {
			return "", nil, err
		}
		execMode = mode
		logger.Info("auto execution mode resolved", "agent_id", agent.ID, "resolved_mode", execMode, "user_input", userInput)
	}

	switch execMode {
	case ExecutionModeReAct:
		return r.runReActLoop(ctx, agent, systemPrompt, userInput, history, tools)
	case ExecutionModePlanExecute:
		s, _, err := r.runPlanAndExecute(ctx, agent, systemPrompt, userInput, history, tools, sessionID, auditUserID)
		return s, nil, err
	default:
		s, err := r.runAgentCore(ctx, agent, systemPrompt, userInput, history, tools)
		return s, nil, err
	}
}

func (r *Runtime) WrapToolsWithAudit(agent *schema.AgentWithRuntime, sessionID, auditUserID string, tools []tool.BaseTool) []tool.BaseTool {
	return r.wrapToolsWithAudit(agent, sessionID, auditUserID, tools)
}

func (r *Runtime) wrapToolsWithAudit(agent *schema.AgentWithRuntime, sessionID, auditUserID string, tools []tool.BaseTool) []tool.BaseTool {
	if r.auditLogger == nil {
		return tools
	}

	approvalMode := ""
	if agent.RuntimeProfile != nil {
		approvalMode = agent.RuntimeProfile.ApprovalMode
	}
	if approvalMode == "" {
		approvalMode = "auto"
	}

	wrapped := make([]tool.BaseTool, 0, len(tools))
	for _, t := range tools {
		if inv, ok := t.(tool.InvokableTool); ok {
			info, err := inv.Info(context.Background())
			if err != nil {
				wrapped = append(wrapped, t)
				continue
			}
			riskLevel := r.resolveToolRiskLevel(info.Name)
			wrapped = append(wrapped, &auditHITLWrapper{
				inner:           inv,
				toolName:        info.Name,
				riskLevel:       riskLevel,
				agentID:         agent.ID,
				sessionID:       sessionID,
				userID:          auditUserID,
				approvalMode:    approvalMode,
				auditLogger:     r.auditLogger,
				approvalChecker: r.buildApprovalChecker(agent, sessionID, auditUserID),
			})
		} else {
			wrapped = append(wrapped, t)
		}
	}
	return wrapped
}

func (r *Runtime) buildApprovalChecker(agent *schema.AgentWithRuntime, sessionID, auditUserID string) func(agentID int64, sessionID, toolName, riskLevel, input string) (bool, int64, error) {
	return func(agentID int64, sessionID, toolName, riskLevel, input string) (bool, int64, error) {
		approvalMode := ""
		approvalType := "internal"
		approvers := []string{}
		approvalTimeout := 30

		if agent.RuntimeProfile != nil {
			approvalMode = agent.RuntimeProfile.ApprovalMode
			approvalType = agent.RuntimeProfile.ApprovalType
			approvers = agent.RuntimeProfile.Approvers
			if agent.RuntimeProfile.ApprovalTimeout > 0 {
				approvalTimeout = agent.RuntimeProfile.ApprovalTimeout
			}
		}
		if approvalMode == "" {
			approvalMode = "auto"
		}
		if approvalMode == "auto" {
			return true, 0, nil
		}

		if r.auditLogger == nil {
			return true, 0, nil
		}

		store, ok := r.auditLogger.store.(interface {
			CreateApprovalRequest(req *agentmodel.ApprovalRequest) (*agentmodel.ApprovalRequest, error)
		})
		if !ok {
			return true, 0, nil
		}

		var expiresAtPtr *time.Time
		if approvalTimeout > 0 {
			t := time.Now().Add(time.Duration(approvalTimeout) * time.Minute)
			expiresAtPtr = &t
		}

		req := &agentmodel.ApprovalRequest{
			AgentID:      agentID,
			SessionID:    sessionID,
			UserID:       auditUserID,
			ToolName:     toolName,
			RiskLevel:    riskLevel,
			Input:        input,
			Status:       "pending",
			CreatedAt:    time.Now(),
			ApprovalType: approvalType,
			ExpiresAt:    expiresAtPtr,
		}

		if approvalType != "internal" && len(approvers) > 0 {
			provider := r.approvalProvider
			if provider != nil {
				externalID, err := provider.SubmitApproval(context.Background(), &approval.ExternalApprovalRequest{
					AgentID:   agentID,
					SessionID: sessionID,
					UserID:    auditUserID,
					ToolName:  toolName,
					RiskLevel: riskLevel,
					Input:     input,
					Approvers: approvers,
					Title:     fmt.Sprintf("审批请求: %s (%s)", toolName, riskLevel),
					Timeout:   time.Duration(approvalTimeout) * time.Minute,
				})
				if err != nil {
					return false, 0, fmt.Errorf("failed to submit external approval: %w", err)
				}
				req.ExternalID = externalID
			}
		}

		createdReq, err := store.CreateApprovalRequest(req)
		if err != nil {
			return false, 0, fmt.Errorf("failed to create approval request: %w", err)
		}

		approvalID := int64(0)
		if createdReq != nil && createdReq.ID > 0 {
			approvalID = createdReq.ID
			r.notifyApprovers(createdReq, approvers)
		}

		return false, approvalID, nil
	}
}

func (r *Runtime) notifyApprovers(req *agentmodel.ApprovalRequest, approvers []string) {
	if len(approvers) == 0 {
		return
	}

	notifier := r.approvalNotifier
	if notifier == nil {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("agent_run: approval notification panic", "recover", r)
			}
		}()
		ctx := context.Background()
		if err := notifier.NotifyApprovalRequest(ctx, req, approvers); err != nil {
			logger.Warn("failed to notify approvers", "err", err)
		}
	}()
}

func (r *Runtime) GateClientToolApproval(agent *schema.AgentWithRuntime, sessionID, auditUserID, toolName, toolArgs string) (blockedPending bool, approvalID int64, err error) {
	riskLevel := r.resolveToolRiskLevel(toolName)
	approvalMode := "auto"
	if agent.RuntimeProfile != nil && strings.TrimSpace(agent.RuntimeProfile.ApprovalMode) != "" {
		approvalMode = strings.TrimSpace(agent.RuntimeProfile.ApprovalMode)
	}
	needApproval := false
	switch approvalMode {
	case "all":
		needApproval = true
	case "high_and_above":
		needApproval = riskLevel == "high" || riskLevel == "critical"
	default:
		needApproval = false
	}
	if !needApproval {
		return false, 0, nil
	}
	checker := r.buildApprovalChecker(agent, sessionID, auditUserID)
	approved, aid, err := checker(agent.ID, sessionID, toolName, riskLevel, toolArgs)
	if err != nil {
		return false, 0, err
	}
	if approved {
		return false, 0, nil
	}
	return true, aid, nil
}

func (r *Runtime) runAgentCoreIterator(ctx context.Context, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, chatTools []tool.BaseTool, visionParts []VisionPart) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	if r.chatModel == nil {
		return nil, fmt.Errorf("chat model not configured")
	}

	instruction := systemPrompt
	if instruction == "" {
		instruction = "You are a helpful AI assistant."
	}
	if _, ok := imoutbound.ScopeFromContext(ctx); ok {
		instruction += imoutbound.IMAgentInstructionSuffix()
	} else {
		instruction += skills.WebAgentInstructionSuffix()
	}
	if len(chatTools) > 0 {
		if hints := r.mcpUsageHintsFromAgent(agent); hints != "" {
			instruction += "\n\n" + hints
		}
		if hints := r.skillUsageHintsFromAgent(ctx, agent); hints != "" {
			instruction += "\n\n" + hints
		}
	}

	msgs := history
	if msgs == nil {
		msgs = make([]*einoschema.Message, 0)
	}
	msgs = append(msgs, buildStreamUserMessage(userInput, visionParts))

	toolsCfg := adk.ToolsConfig{}
	if len(chatTools) > 0 {
		toolsCfg = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: chatTools},
		}
	}

	adkDesc := strings.TrimSpace(agent.Desc)
	if adkDesc == "" {
		adkDesc = strings.TrimSpace(agent.Name)
	}
	if adkDesc == "" {
		adkDesc = "Assistant"
	}

	agentImpl, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          agent.Name,
		Description:   adkDesc,
		Instruction:   instruction,
		Model:         r.chatModel,
		ToolsConfig:   toolsCfg,
		MaxIterations: 32,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agentImpl,
		EnableStreaming: true,
	})

	return runner.Run(ctx, msgs), nil
}

func emitToolCallsFromMessage(
	w io.Writer,
	msg *einoschema.Message,
	chatTools []tool.BaseTool,
	agent *schema.AgentWithRuntime,
	clientType string,
	ts func() string,
) error {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return nil
	}
	for _, tc := range msg.ToolCalls {
		name := tc.Function.Name
		if name == "" {
			name = "tool"
		}
		execMode := getToolExecutionModeFromTools(chatTools, name, skillExecOverrides(agent))
		logger.Info("emit tool call", "tool_name", name, "exec_mode", execMode, "client_type", clientType)
		if execMode == schema.ExecutionModeClient && clientType != "desktop" {
			logger.Warn("tool not supported on web", "tool_name", name, "exec_mode", execMode, "client_type", clientType)
			if err := writeSSEJSONEvent(w, map[string]any{
				"event_type": "info",
				"content":    fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", name),
				"timestamp":  ts(),
			}); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
				return err
			}
			return errClientToolUnsupportedWeb
		}
		var input map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		if input == nil {
			input = map[string]any{}
		}
		if err := writeSSEJSONEvent(w, map[string]any{
			"event_type": "tool_call",
			"tool_name":  name,
			"name":       name,
			"input":      input,
			"timestamp":  ts(),
		}); err != nil {
			return err
		}
	}
	return nil
}

var errClientToolUnsupportedWeb = errors.New("client tool unsupported on web")

func (r *Runtime) runAgentCoreStream(ctx context.Context, w io.Writer, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, chatTools []tool.BaseTool, clientType string, visionParts []VisionPart) error {
	logger.Info("chat stream adk", "phase", "start", "agent_id", agent.ID, "tools", len(chatTools), "history", len(history), "vision", len(visionParts), "client_type", clientType)
	ctx = skills.WithAttachmentCollector(ctx)
	iter, err := r.runAgentCoreIterator(ctx, agent, systemPrompt, userInput, history, chatTools, visionParts)
	if err != nil {
		logger.Warn("chat stream adk", "phase", "iterator_open", "agent_id", agent.ID, "err", err)
		return err
	}
	var lastFinal string
	ts := func() string { return time.Now().Format(time.RFC3339Nano) }
	events := 0
	for {
		event, ok := iter.Next()
		if !ok {
			logger.Info("chat stream adk", "phase", "iterator_exhausted", "agent_id", agent.ID, "events", events, "last_final_len", len(strings.TrimSpace(lastFinal)))
			break
		}
		events++
		if event.Err != nil {
			logger.Warn("chat stream adk", "phase", "iterator", "agent_id", agent.ID, "event_n", events, "err", event.Err)
			return event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput

		if mv.IsStreaming && mv.MessageStream != nil {
			mv.MessageStream.SetAutomaticClose()
			var streamBuf strings.Builder
			chunks := 0
			for {
				chunk, rerr := mv.MessageStream.Recv()
				if errors.Is(rerr, io.EOF) {
					logger.Info("chat stream adk", "phase", "message_stream_eof", "chunks", chunks, "acc_bytes", streamBuf.Len())
					break
				}
				if rerr != nil {
					logger.Warn("chat stream adk", "phase", "message_stream_recv", "agent_id", agent.ID, "err", rerr)
					return rerr
				}
				if chunk == nil {
					continue
				}
				chunks++
				logger.Debug("chat stream adk", "phase", "stream_recv", "n", chunks,
					"content_len", len(chunk.Content), "reasoning_len", len(chunk.ReasoningContent), "tool_calls", len(chunk.ToolCalls))
				if err := emitToolCallsFromMessage(w, chunk, chatTools, agent, clientType, ts); err != nil {
					if errors.Is(err, errClientToolUnsupportedWeb) {
						return nil
					}
					return err
				}
				out := assistantChunkText(chunk)
				if out != "" {
					streamBuf.WriteString(out)
					if err := streamStaticTextAsSSE(w, out); err != nil {
						return err
					}
				}
			}
			mv.MessageStream.Close()
			if streamBuf.Len() > 0 {
				lastFinal = streamBuf.String()
			}
			continue
		}

		msg, err := mv.GetMessage()
		if err != nil {
			logger.Warn("chat stream adk", "phase", "get_message", "agent_id", agent.ID, "err", err)
			return err
		}
		if err := emitToolCallsFromMessage(w, msg, chatTools, agent, clientType, ts); err != nil {
			if errors.Is(err, errClientToolUnsupportedWeb) {
				return nil
			}
			return err
		}
		role := msg.Role
		if role == "" {
			role = mv.Role
		}
		logger.Debug("chat stream adk", "phase", "get_message", "role", role, "content_len", len(msg.Content), "tool_calls", len(msg.ToolCalls))
		if role == einoschema.Tool {
			toolName := msg.ToolName
			if toolName == "" {
				toolName = mv.ToolName
			}
			if toolName == "" {
				toolName = "tool"
			}
			resultData := map[string]any{
				"event_type": "tool_result",
				"tool_name":  toolName,
				"result":     msg.Content,
				"timestamp":  ts(),
			}
			if enriched := skills.EnrichScreenshotToolResult(msg.Content); enriched != msg.Content {
				resultData["result"] = enriched
			}
			if attachments := skills.ConsumeAttachments(ctx); len(attachments) > 0 {
				resultData["attachments"] = attachments
			}
			if err := writeSSEJSONEvent(w, resultData); err != nil {
				return err
			}
		}
		if msg.Content != "" && len(msg.ToolCalls) == 0 {
			lastFinal = msg.Content
			if err := streamStaticTextAsSSE(w, msg.Content); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(lastFinal) == "" {
		logger.Warn("chat stream adk", "phase", "finish", "agent_id", agent.ID, "reason", "empty_last_final", "events", events)
		return fmt.Errorf("no response from agent")
	}
	logger.Info("chat stream adk", "phase", "finish", "ok", true, "agent_id", agent.ID, "last_len", len(lastFinal))
	return nil
}

func (r *Runtime) runAgentCore(ctx context.Context, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, chatTools []tool.BaseTool) (string, error) {
	iter, err := r.runAgentCoreIterator(ctx, agent, systemPrompt, userInput, history, chatTools, nil)
	if err != nil {
		return "", err
	}
	var lastFinal string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				return "", err
			}
			if msg.Content != "" && len(msg.ToolCalls) == 0 {
				lastFinal = msg.Content
			}
		}
	}

	if strings.TrimSpace(lastFinal) != "" {
		return lastFinal, nil
	}
	return "", fmt.Errorf("no response from agent")
}

func buildSystemPrompt(profile *schema.RuntimeProfile) string {
	if profile == nil {
		return ""
	}

	var parts []string

	if profile.Role != "" {
		parts = append(parts, "Role: "+profile.Role)
	}
	if profile.Goal != "" {
		parts = append(parts, "Goal: "+profile.Goal)
	}
	if profile.Backstory != "" {
		parts = append(parts, "Backstory: "+profile.Backstory)
	}

	return strings.Join(parts, "\n")
}

func truncateRunesForPlanDetail(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}

func summarizeToolResultForPlanDetail(result string) string {
	s := strings.TrimSpace(result)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "\n--- extracted text for model ---"); idx >= 0 {
		s = s[:idx]
	}
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, 4)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kept = append(kept, line)
		if len(kept) >= 4 {
			break
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return truncateRunesForPlanDetail(strings.Join(kept, " | "), 320)
}
