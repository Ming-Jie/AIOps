package agent

import (
	"context"
	"fmt"

	einoschema "github.com/cloudwego/eino/schema"
)

func (r *Runtime) ChatWithMemoryContext(ctx context.Context, agentID int64, message, imageBase64, imageMime string, memContext string, sessionID, auditUserID string, historyMsgs []*einoschema.Message) (string, error) {
	ctx = contextWithAgentID(ctx, agentID)
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return "", fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = r.cachedSystemPrompt(agentID, agent.RuntimeProfile)
	}

	historyWithMemory := historyMsgs
	memContext = r.mergeKBContext(ctx, agentID, message, memContext)
	if memContext != "" {
		historyWithMemory = append([]*einoschema.Message{
			{Role: einoschema.User, Content: "[记忆上下文]\n" + memContext},
		}, historyMsgs...)
	}

	resp, _, err := r.runAgent(ctx, agent, systemPrompt, message, historyWithMemory, sessionID, auditUserID)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (r *Runtime) ChatWithMemoryContextSchedule(ctx context.Context, agentID int64, message, imageBase64, imageMime string, memContext string, sessionID, auditUserID string, historyMsgs []*einoschema.Message) (string, []map[string]any, error) {
	ctx = contextWithAgentID(ctx, agentID)
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return "", nil, fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)
	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = r.cachedSystemPrompt(agentID, agent.RuntimeProfile)
	}
	historyWithMemory := historyMsgs
	memContext = r.mergeKBContext(ctx, agentID, message, memContext)
	if memContext != "" {
		historyWithMemory = append([]*einoschema.Message{
			{Role: einoschema.User, Content: "[记忆上下文]\n" + memContext},
		}, historyMsgs...)
	}
	resp, reactRes, err := r.runAgent(ctx, agent, systemPrompt, message, historyWithMemory, sessionID, auditUserID)
	if err != nil {
		return "", nil, err
	}
	execMode := ExecutionModeDefault
	if agent.RuntimeProfile != nil {
		execMode = agent.RuntimeProfile.ExecutionMode
	}
	if execMode != ExecutionModeReAct {
		return resp, nil, nil
	}
	payloads := ReActResultToReactPayloads(reactRes, resp)
	return resp, payloads, nil
}

func (r *Runtime) ChatWithUserProfile(ctx context.Context, agentID int64, message, imageBase64, imageMime, userProfile string, sessionID, auditUserID string, historyMsgs []*einoschema.Message) (string, error) {
	ctx = contextWithAgentID(ctx, agentID)
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return "", fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = r.cachedSystemPrompt(agentID, agent.RuntimeProfile)
	}

	if userProfile != "" {
		systemPrompt = userProfile + "\n\n" + systemPrompt
	}

	resp, _, err := r.runAgent(ctx, agent, systemPrompt, message, historyMsgs, sessionID, auditUserID)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (r *Runtime) Chat(ctx context.Context, agentID int64, message, sessionID, auditUserID string) (string, error) {
	return r.ChatWithMemoryContext(ctx, agentID, message, "", "", "", sessionID, auditUserID, nil)
}
