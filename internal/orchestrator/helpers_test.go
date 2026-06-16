package orchestrator

import (
	"strings"
	"testing"

	"github.com/fisk086/aiops/internal/schema"
)

func TestAgentDisplayLabel_prefersRole(t *testing.T) {
	awr := &schema.AgentWithRuntime{
		Agent: schema.Agent{Name: "dba"},
		RuntimeProfile: &schema.RuntimeProfile{
			Role: "数据库管理员",
		},
	}
	if got := agentDisplayLabel(awr); got != "数据库管理员" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildMemberExecutionPrompt_identityConstraint(t *testing.T) {
	o := &Orchestrator{}
	awr := &schema.AgentWithRuntime{
		Agent: schema.Agent{Name: "cloud"},
		RuntimeProfile: &schema.RuntimeProfile{
			Role: "Cloud工程师",
			Goal: "负责云资源",
		},
	}
	prompt := o.buildMemberExecutionPrompt(awr, "协调员分配方案：...")
	if !strings.Contains(prompt, "Cloud工程师") {
		t.Fatalf("missing role in prompt: %s", prompt)
	}
	if !strings.Contains(prompt, "严禁冒充") {
		t.Fatalf("missing identity constraint: %s", prompt)
	}
}
