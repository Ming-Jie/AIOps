package service

import (
	"context"
	"strings"
	"testing"

	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

func addTestAgent(t *testing.T, store *storage.InMemoryStorage, name string) int64 {
	t.Helper()
	ag, err := store.CreateAgent(&schema.CreateAgentRequest{
		Name:        name,
		Description: name + " desc",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return ag.ID
}

func TestResolveGroupInvokeAgentIDsEmptyMentionsUsesAllMembers(t *testing.T) {
	store := storage.NewInMemoryStorage()
	a1 := addTestAgent(t, store, "alpha")
	a2 := addTestAgent(t, store, "beta")

	group, err := store.CreateChatGroup(context.Background(), &schema.CreateGroupRequest{
		Name:     "g",
		AgentIDs: []int64{a1, a2},
	}, "u")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	svc := &ChatService{store: store}
	got := svc.resolveGroupInvokeAgentIDs(context.Background(), group.ID, nil)
	want := []int64{a1, a2}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestResolveGroupInvokeAgentIDsFiltersMentionsToGroupMembers(t *testing.T) {
	store := storage.NewInMemoryStorage()
	a1 := addTestAgent(t, store, "alpha")
	a2 := addTestAgent(t, store, "beta")
	outsider := addTestAgent(t, store, "outside")

	group, err := store.CreateChatGroup(context.Background(), &schema.CreateGroupRequest{
		Name:     "g",
		AgentIDs: []int64{a1, a2},
	}, "u")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	svc := &ChatService{store: store}
	got := svc.resolveGroupInvokeAgentIDs(context.Background(), group.ID, []int64{outsider, a2, a2})
	if len(got) != 1 || got[0] != a2 {
		t.Fatalf("got %v want [%d]", got, a2)
	}
}

func TestGroupCollaborationMessage(t *testing.T) {
	got := groupCollaborationMessage("排查报警", 3)
	if !strings.Contains(got, "群聊协作模式") || !strings.Contains(got, "排查报警") {
		t.Fatalf("unexpected collaboration message: %q", got)
	}

	single := groupCollaborationMessage("hello", 1)
	if single != "hello" {
		t.Fatalf("single member message changed: %q", single)
	}
}

func TestParseGroupMultiplexSSEToAssistantTextFinalAnswer(t *testing.T) {
	raw := []byte(`data: {"agent_id":1,"type":"final_answer","content":"ok"}` + "\n\n")
	got := parseGroupMultiplexSSEToAssistantText(raw)
	if got[1] != "ok" {
		t.Fatalf("got %#v want final answer", got)
	}
}
