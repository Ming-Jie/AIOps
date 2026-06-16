package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/schema"
)

// ExecuteAgentFn is a bridge to the agent runtime's chat capability
var ExecuteAgentFn = func(ctx context.Context, awr *schema.AgentWithRuntime, rt *agent.Runtime, prompt string) (string, error) {
	resp, err := rt.Chat(ctx, awr.ID, prompt, "", "")
	if err != nil {
		return "", err
	}
	return resp, nil
}

type TeamStore interface {
	GetTeamWithMembers(ctx context.Context, teamID int64) (*model.Team, []*model.TeamMember, error)
	CreateConversation(ctx context.Context, conv *model.TeamConversation) (*model.TeamConversation, error)
	GetConversation(ctx context.Context, id int64) (*model.TeamConversation, error)
	UpdateConversation(ctx context.Context, conv *model.TeamConversation) error
	CreateMessage(ctx context.Context, msg *model.TeamMessage) (*model.TeamMessage, error)
	ListMessagesByConversation(ctx context.Context, convID int64) ([]*model.TeamMessage, error)
}

type Orchestrator struct {
	mu        sync.RWMutex
	store     TeamStore
	runtime   *agent.Runtime
	agents    map[int64]*schema.AgentWithRuntime
}

func NewOrchestrator(store TeamStore, runtime *agent.Runtime) *Orchestrator {
	return &Orchestrator{
		store:   store,
		runtime: runtime,
		agents:  make(map[int64]*schema.AgentWithRuntime),
	}
}

func (o *Orchestrator) RegisterAgent(awr *schema.AgentWithRuntime) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.agents[awr.ID] = awr
}

func (o *Orchestrator) UnregisterAgent(agentID int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.agents, agentID)
}

func (o *Orchestrator) StartConversation(ctx context.Context, teamID int64, title string, userID int64) (*model.TeamConversation, error) {
	_, _, err := o.store.GetTeamWithMembers(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	conv := &model.TeamConversation{
		TeamID:    teamID,
		Title:     title,
		Status:    "active",
		StartedBy: userID,
		Round:     0,
	}
	return o.store.CreateConversation(ctx, conv)
}

func (o *Orchestrator) SendMessage(ctx context.Context, convID int64, text string) (*schema.TeamConversationResponse, error) {
	conv, err := o.store.GetConversation(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	team, members, err := o.store.GetTeamWithMembers(ctx, conv.TeamID)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	var activeMembers []*model.TeamMember
	for _, m := range members {
		if m.IsActive {
			activeMembers = append(activeMembers, m)
		}
	}

	if len(activeMembers) == 0 {
		return nil, fmt.Errorf("team has no active members")
	}

	conv.Round++
	if conv.Round > team.MaxRounds {
		conv.Status = "completed"
		o.store.UpdateConversation(ctx, conv)
		return nil, fmt.Errorf("max rounds (%d) reached", team.MaxRounds)
	}
	o.store.UpdateConversation(ctx, conv)

	userMsg := &model.TeamMessage{
		ConversationID: convID,
		TeamID:         team.ID,
		SenderAgentID:  0,
		SenderName:     "User",
		Content:        text,
		MsgType:        "user_input",
		Round:          conv.Round,
	}
	o.store.CreateMessage(ctx, userMsg)

	switch team.Mode {
	case model.TeamModeGroupChat:
		_, err = o.runGroupChat(ctx, conv, team, activeMembers, text)
	case model.TeamModeDebate:
		_, err = o.runDebate(ctx, conv, team, activeMembers, text)
	case model.TeamModeRouting:
		_, err = o.runRouting(ctx, conv, team, activeMembers, text)
	case model.TeamModeSequential:
		_, err = o.runSequential(ctx, conv, team, activeMembers, text)
	default:
		_, err = o.runGroupChat(ctx, conv, team, activeMembers, text)
	}

	if err != nil {
		logger.Error("orchestrator: execution failed", "team_id", team.ID, "mode", team.Mode, "err", err)
	}

	allMsgs, _ := o.store.ListMessagesByConversation(ctx, convID)

	return o.convertToResponse(conv, allMsgs), nil
}

func (o *Orchestrator) runGroupChat(ctx context.Context, conv *model.TeamConversation, team *model.Team, members []*model.TeamMember, userInput string) ([]*model.TeamMessage, error) {
	context := o.buildContext(conv, userInput)

	var coordinator *model.TeamMember
	var agents []*model.TeamMember
	for _, m := range members {
		if team.CoordinatorAgentID != nil && m.AgentID == *team.CoordinatorAgentID {
			coordinator = m
		} else {
			agents = append(agents, m)
		}
	}

	if coordinator == nil && len(members) > 0 {
		coordinator = members[0]
		if len(agents) == 0 && len(members) > 1 {
			agents = members[1:]
		}
	}

	var result []*model.TeamMessage

	if coordinator != nil {
		roster := o.buildTeamMemberRoster(members)
		planPrompt := fmt.Sprintf(`你是团队协调者。以下是团队成员（分配任务时必须使用下列名称/角色，勿混淆代号与职责）：
%s

用户请求：%s

请分析任务并为需要参与的每位成员分别列出职责。使用 Markdown 三级标题「### 成员名或角色」，每位成员只分配与其角色匹配的任务，不要把 A 成员的任务交给 B 成员。`, roster, userInput)
		planResp, err := o.callAgent(ctx, conv, coordinator.AgentID, planPrompt)
		if err == nil {
			planMsg := &model.TeamMessage{
				ConversationID: conv.ID,
				TeamID:         team.ID,
				SenderAgentID:  coordinator.AgentID,
				SenderName:     planResp.name,
				Content:        fmt.Sprintf("**任务分配**:\n%s", planResp.content),
				MsgType:        "coordination",
				Round:          conv.Round,
			}
			o.store.CreateMessage(ctx, planMsg)
			result = append(result, planMsg)
			context = fmt.Sprintf("%s\n\n协调员分配方案:\n%s\n\n请根据分配执行你的任务。", context, planResp.content)
		}
	}

	for _, member := range agents {
		awr := o.getAgentByID(member.AgentID)
		memberContext := o.buildMemberExecutionPrompt(awr, context)

		resp, err := o.callAgent(ctx, conv, member.AgentID, memberContext)
		if err != nil {
			content := fmt.Sprintf("（%s 暂时无法响应）", o.getAgentDisplayName(member.AgentID))
			errMsg := &model.TeamMessage{
				ConversationID: conv.ID,
				TeamID:         team.ID,
				SenderAgentID:  member.AgentID,
				SenderName:     o.getAgentDisplayName(member.AgentID),
				Content:        content,
				MsgType:        "error",
				Round:          conv.Round,
			}
			o.store.CreateMessage(ctx, errMsg)
			result = append(result, errMsg)
			continue
		}

		msg := &model.TeamMessage{
			ConversationID: conv.ID,
			TeamID:         team.ID,
			SenderAgentID:  member.AgentID,
			SenderName:     resp.name,
			Content:        resp.content,
			MsgType:        "message",
			Round:          conv.Round,
		}
		o.store.CreateMessage(ctx, msg)
		result = append(result, msg)
	}

	if coordinator != nil {
		summaryPrompt := fmt.Sprintf("请总结团队成员对以下用户请求的回复。\n\n用户请求: %s\n\n团队成员回复:\n", userInput)
		for _, msg := range result {
			if msg.MsgType == "message" {
				summaryPrompt += fmt.Sprintf("\n--- %s ---\n%s\n", msg.SenderName, imoutbound.ContentForSummary(msg.Content))
			}
		}
		summaryPrompt += `

请给出综合文字总结。不要重复附带 [[im_file:...]] 截图/文件标记；成员回复中已包含的附件无需在总结中再次出现，用文字说明即可。`

		summaryResp, err := o.callAgent(ctx, conv, coordinator.AgentID, summaryPrompt)
		if err == nil {
			summaryContent := imoutbound.StripFileMarkers(summaryResp.content)
			summaryMsg := &model.TeamMessage{
				ConversationID: conv.ID,
				TeamID:         team.ID,
				SenderAgentID:  coordinator.AgentID,
				SenderName:     summaryResp.name,
				Content:        summaryContent,
				MsgType:        "summary",
				Round:          conv.Round,
			}
			o.store.CreateMessage(ctx, summaryMsg)
			result = append(result, summaryMsg)
		}
	}

	return result, nil
}

func (o *Orchestrator) runDebate(ctx context.Context, conv *model.TeamConversation, team *model.Team, members []*model.TeamMember, userInput string) ([]*model.TeamMessage, error) {
	context := o.buildContext(conv, userInput)

	var result []*model.TeamMessage

	rounds := 1
	if v, ok := team.Config["debate_rounds"]; ok {
		if r, ok := v.(float64); ok {
			rounds = int(r)
		}
	}

	for round := 1; round <= rounds; round++ {
		roundLabel := ""
		if round == 1 {
			roundLabel = "首轮陈述"
		} else if round == rounds {
			roundLabel = "总结陈词"
		} else {
			roundLabel = fmt.Sprintf("第%d轮辩论", round)
		}

		for _, member := range members {
			awr := o.getAgentByID(member.AgentID)
			roundCtx := fmt.Sprintf("%s\n\n当前轮次: %s", context, roundLabel)
			if stance := o.getMemberStance(member, team.Config); stance != "" {
				roundCtx += "\n观点立场: " + stance
			}
			memberPrompt := o.buildMemberExecutionPrompt(awr, roundCtx)

			resp, err := o.callAgent(ctx, conv, member.AgentID, memberPrompt)
			if err != nil {
				continue
			}

			msg := &model.TeamMessage{
				ConversationID: conv.ID,
				TeamID:         team.ID,
				SenderAgentID:  member.AgentID,
				SenderName:     resp.name,
				Content:        fmt.Sprintf("**【%s】**\n\n%s", roundLabel, resp.content),
				MsgType:        "debate",
				Round:          conv.Round,
				Metadata:       map[string]any{"debate_round": round},
			}
			o.store.CreateMessage(ctx, msg)
			result = append(result, msg)

			context = fmt.Sprintf("%s\n\n%s(%s): %s", context, resp.name, roundLabel, resp.content)
		}
	}

	consensusPrompt := fmt.Sprintf("请综合以上辩论观点，给出最终结论，指出共识和分歧点。\n\n原始议题: %s", userInput)
	if coordinatorID := team.CoordinatorAgentID; coordinatorID != nil {
		resp, err := o.callAgent(ctx, conv, *coordinatorID, consensusPrompt)
		if err == nil {
			msg := &model.TeamMessage{
				ConversationID: conv.ID,
				TeamID:         team.ID,
				SenderAgentID:  *coordinatorID,
				SenderName:     resp.name,
				Content:        fmt.Sprintf("**辩论结论**:\n\n%s", resp.content),
				MsgType:        "summary",
				Round:          conv.Round,
			}
			o.store.CreateMessage(ctx, msg)
			result = append(result, msg)
		}
	}

	return result, nil
}

func (o *Orchestrator) runRouting(ctx context.Context, conv *model.TeamConversation, team *model.Team, members []*model.TeamMember, userInput string) ([]*model.TeamMessage, error) {
	routingPrompt := "你是一个智能路由分发器。有以下可用智能体:\n"

	for _, m := range members {
		awr := o.getAgentByID(m.AgentID)
		if awr != nil {
			desc := awr.Desc
			if desc == "" && awr.RuntimeProfile != nil {
				desc = strings.TrimSpace(awr.RuntimeProfile.Goal)
			}
			if desc == "" {
				desc = awr.Name
			}
			label := agentDisplayLabel(awr)
			routingPrompt += fmt.Sprintf("- %s (ID: %d", label, awr.ID)
			if code := strings.TrimSpace(awr.Name); code != "" && code != label {
				routingPrompt += fmt.Sprintf(", 代号 %s", code)
			}
			routingPrompt += fmt.Sprintf("): %s\n", desc)
		}
	}
	routingPrompt += fmt.Sprintf("\n用户请求: %s\n\n请选择最合适的智能体来处理此请求，只返回智能体ID和简短理由。", userInput)

	var selectedAgentID int64
	var selectedReason string

	if leader := o.getCoordinator(team, members); leader != nil {
		awr := o.getAgentByID(leader.AgentID)
		if awr != nil {
			resp, err := o.callAgent(ctx, conv, leader.AgentID, routingPrompt)
			if err == nil {
				content := resp.content
				idStr := extractDigits(content)
				if idStr != "" {
					fmt.Sscanf(idStr, "%d", &selectedAgentID)
					selectedReason = content
				}

				routeMsg := &model.TeamMessage{
					ConversationID: conv.ID,
					TeamID:         team.ID,
					SenderAgentID:  leader.AgentID,
					SenderName:     resp.name,
					Content:        fmt.Sprintf("**路由决策**: 选择智能体处理\n\n理由: %s", content),
					MsgType:        "routing",
					Round:          conv.Round,
				}
				o.store.CreateMessage(ctx, routeMsg)
			}
		}
	}

	if selectedAgentID == 0 {
		selectedAgentID = members[0].AgentID
		selectedReason = "使用默认智能体"
	}

	var result []*model.TeamMessage

	for _, m := range members {
		if m.AgentID == selectedAgentID {
			awr := o.getAgentByID(m.AgentID)
			memberContext := o.buildMemberExecutionPrompt(awr,
				fmt.Sprintf("你已被选中处理以下请求:\n\n%s\n\n路由理由: %s", userInput, selectedReason))

			resp, err := o.callAgent(ctx, conv, m.AgentID, memberContext)
			if err != nil {
				continue
			}

			msg := &model.TeamMessage{
				ConversationID: conv.ID,
				TeamID:         team.ID,
				SenderAgentID:  m.AgentID,
				SenderName:     resp.name,
				Content:        resp.content,
				MsgType:        "message",
				Round:          conv.Round,
			}
			o.store.CreateMessage(ctx, msg)
			result = append(result, msg)
		}
	}

	return result, nil
}

func (o *Orchestrator) runSequential(ctx context.Context, conv *model.TeamConversation, team *model.Team, members []*model.TeamMember, userInput string) ([]*model.TeamMessage, error) {
	context := userInput
	var result []*model.TeamMessage

	for i, member := range members {
		awr := o.getAgentByID(member.AgentID)
		stepCtx := fmt.Sprintf("这是流程中的第 %d/%d 步。\n\n当前输入:\n%s\n\n请根据你的角色处理以上内容并输出结果。",
			i+1, len(members), context)
		memberContext := o.buildMemberExecutionPrompt(awr, stepCtx)

		resp, err := o.callAgent(ctx, conv, member.AgentID, memberContext)
		if err != nil {
			content := fmt.Sprintf("（步骤 %d 处理失败: %v）", i+1, err)
			errMsg := &model.TeamMessage{
				ConversationID: conv.ID,
				TeamID:         team.ID,
				SenderAgentID:  member.AgentID,
				SenderName:     o.getAgentDisplayName(member.AgentID),
				Content:        content,
				MsgType:        "error",
				Round:          conv.Round,
			}
			o.store.CreateMessage(ctx, errMsg)
			result = append(result, errMsg)
			break
		}

		stepLabel := fmt.Sprintf("**步骤 %d - %s**", i+1, resp.name)
		msg := &model.TeamMessage{
			ConversationID: conv.ID,
			TeamID:         team.ID,
			SenderAgentID:  member.AgentID,
			SenderName:     resp.name,
			Content:        fmt.Sprintf("%s\n\n%s", stepLabel, resp.content),
			MsgType:        "message",
			Round:          conv.Round,
			Metadata:       map[string]any{"step": i + 1, "total_steps": len(members)},
		}
		o.store.CreateMessage(ctx, msg)
		result = append(result, msg)

		context = resp.content
	}

	return result, nil
}

type agentCallResult struct {
	name    string
	content string
}

func (o *Orchestrator) callAgent(ctx context.Context, conv *model.TeamConversation, agentID int64, prompt string) (*agentCallResult, error) {
	awr := o.getAgentByID(agentID)
	if awr == nil {
		return nil, fmt.Errorf("agent %d not found in orchestrator", agentID)
	}

	agentName := agentDisplayLabel(awr)
	if agentName == "" {
		agentName = awr.Name
	}

	if conv != nil && conv.ID > 0 && agentID > 0 {
		ctx = imoutbound.WithScope(ctx, agentID, imoutbound.TeamConvSessionID(conv.ID))
	}

	resp, err := ExecuteAgentFn(ctx, awr, o.runtime, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent call failed: %w", err)
	}

	if resp == "" {
		resp = fmt.Sprintf("（智能体 %s 已处理请求）", agentName)
	}

	return &agentCallResult{name: agentName, content: resp}, nil
}

func (o *Orchestrator) buildContext(conv *model.TeamConversation, userInput string) string {
	return fmt.Sprintf("团队对话 (ID: %d, 轮次: %d)\n用户输入: %s",
		conv.ID, conv.Round, userInput)
}

func (o *Orchestrator) getAgentByID(id int64) *schema.AgentWithRuntime {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.agents[id]
}

func (o *Orchestrator) getAgentName(id int64) string {
	return o.getAgentDisplayName(id)
}

func (o *Orchestrator) getCoordinator(team *model.Team, members []*model.TeamMember) *model.TeamMember {
	if team.CoordinatorAgentID != nil {
		for _, m := range members {
			if m.AgentID == *team.CoordinatorAgentID {
				return m
			}
		}
	}
	if len(members) > 0 {
		return members[0]
	}
	return nil
}

func (o *Orchestrator) getMemberStance(member *model.TeamMember, config map[string]any) string {
	if config == nil {
		return ""
	}
	stances, ok := config["stances"]
	if !ok {
		return ""
	}
	stanceMap, ok := stances.(map[string]interface{})
	if !ok {
		return ""
	}
	if stance, ok := stanceMap[fmt.Sprintf("%d", member.AgentID)]; ok {
		if s, ok := stance.(string); ok {
			return s
		}
	}
	return ""
}

func (o *Orchestrator) convertToResponse(conv *model.TeamConversation, msgs []*model.TeamMessage) *schema.TeamConversationResponse {
	resp := &schema.TeamConversationResponse{
		ID:        conv.ID,
		TeamID:    conv.TeamID,
		Title:     conv.Title,
		Status:    conv.Status,
		StartedBy: conv.StartedBy,
		Round:     conv.Round,
		CreatedAt: conv.CreatedAt.Format(time.RFC3339),
		UpdatedAt: conv.UpdatedAt.Format(time.RFC3339),
	}

	for _, m := range msgs {
		resp.Messages = append(resp.Messages, &schema.TeamMsgResp{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			SenderAgentID:  m.SenderAgentID,
			SenderName:     m.SenderName,
			Content:        m.Content,
			MsgType:        m.MsgType,
			TargetAgentID:  m.TargetAgentID,
			Round:          m.Round,
			Metadata:       m.Metadata,
			CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		})
	}

	return resp
}

func extractDigits(s string) string {
	var result strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result.WriteRune(c)
		} else if result.Len() > 0 {
			break
		}
	}
	return result.String()
}

// ExecuteAgent is a bridge to the agent runtime's chat capability
var ExecuteAgent = func(ctx context.Context, awr *schema.AgentWithRuntime, rt *agent.Runtime, prompt string) (string, error) {
	resp, err := rt.Chat(ctx, awr.ID, prompt, "", "")
	if err != nil {
		return "", err
	}
	return resp, nil
}
