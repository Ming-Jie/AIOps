package orchestrator

import (
	"fmt"
	"strings"

	"github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/schema"
)

// agentDisplayLabel prefers runtime role for team UI; falls back to agent name.
func agentDisplayLabel(awr *schema.AgentWithRuntime) string {
	if awr == nil {
		return ""
	}
	if awr.RuntimeProfile != nil {
		if role := strings.TrimSpace(awr.RuntimeProfile.Role); role != "" {
			return role
		}
	}
	return strings.TrimSpace(awr.Name)
}

func (o *Orchestrator) getAgentDisplayName(id int64) string {
	if label := agentDisplayLabel(o.getAgentByID(id)); label != "" {
		return label
	}
	return fmt.Sprintf("Agent(%d)", id)
}

func (o *Orchestrator) buildTeamMemberRoster(members []*model.TeamMember) string {
	var b strings.Builder
	for _, m := range members {
		awr := o.getAgentByID(m.AgentID)
		if awr == nil {
			b.WriteString(fmt.Sprintf("- Agent(%d)\n", m.AgentID))
			continue
		}
		label := agentDisplayLabel(awr)
		line := fmt.Sprintf("- %s", label)
		if code := strings.TrimSpace(awr.Name); code != "" && code != label {
			line += fmt.Sprintf("（代号 %s）", code)
		}
		if awr.RuntimeProfile != nil && strings.TrimSpace(awr.RuntimeProfile.Goal) != "" {
			line += "：" + strings.TrimSpace(awr.RuntimeProfile.Goal)
		} else if desc := strings.TrimSpace(awr.Desc); desc != "" {
			line += "：" + desc
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimSpace(b.String())
}

func (o *Orchestrator) buildMemberExecutionPrompt(awr *schema.AgentWithRuntime, baseContext string) string {
	label := agentDisplayLabel(awr)
	if label == "" {
		label = "团队成员"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【身份约束】你是团队成员「%s」。你必须以该身份回复，严禁冒充其他成员。\n", label))
	if awr != nil && strings.TrimSpace(awr.Name) != "" && awr.Name != label {
		b.WriteString(fmt.Sprintf("你的智能体代号：%s\n", strings.TrimSpace(awr.Name)))
	}
	if awr != nil && awr.RuntimeProfile != nil {
		if role := strings.TrimSpace(awr.RuntimeProfile.Role); role != "" {
			b.WriteString("你的角色：" + role + "\n")
		}
		if goal := strings.TrimSpace(awr.RuntimeProfile.Goal); goal != "" {
			b.WriteString("你的目标：" + goal + "\n")
		}
	}
	b.WriteString(fmt.Sprintf("请**仅**执行协调员方案中分配给「%s」的任务；若方案未提及你，请简短说明本回合无需你执行。\n\n", label))
	b.WriteString(baseContext)
	return b.String()
}
