package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/workspace"
)

func TestWorkspaceAndIntentExpansionTraceDoesNotStoreBodies(t *testing.T) {
	root := t.TempDir()
	wsStore, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	ws, err := wsStore.Create(workspace.Write{
		Type: workspace.TypeProject, Status: workspace.StatusInProgress,
		Title: "T2 context", Body: "SECRET_WORKSPACE_BODY",
	})
	if err != nil {
		t.Fatal(err)
	}
	intentStore, err := intent.OpenStore(filepath.Join(root, "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = intentStore.Close() })
	it, err := intentStore.Create(context.Background(), intent.CreateInput{
		AgentID: identity.LocalCLI().AgentID, Kind: intent.IntentOneShot,
		Title: "T2 intent", Body: "SECRET_INTENT_BODY", DueAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	all, err := All(Deps{
		Scope: identity.LocalCLI(), Episodes: episodes,
		Workspace: wsStore, Intents: intentStore, RecallMode: "agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, collector := observe.WithTaskContextHooks(context.Background(), observe.TaskContextHooks{})
	workspaceOut, err := findInvokableTool(t, all, "read_workspace").InvokableRun(ctx, `{"id":"`+ws.ID+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	intentOut, err := findInvokableTool(t, all, "read_intent").InvokableRun(ctx, `{"id":"`+it.ID+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(workspaceOut, "SECRET_WORKSPACE_BODY") || !strings.Contains(intentOut, "SECRET_INTENT_BODY") {
		t.Fatal("read tools did not return requested bodies")
	}
	events := collector.Expansions()
	if len(events) != 2 || events[0].Source != "workspace" || events[0].ID != ws.ID || events[1].Source != "intent" || events[1].ID != it.ID {
		t.Fatalf("unexpected expansion events: %+v", events)
	}
	raw, err := json.Marshal(observe.TurnTrace{ContextExpands: events})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SECRET_WORKSPACE_BODY") || strings.Contains(string(raw), "SECRET_INTENT_BODY") {
		t.Fatalf("expanded body leaked into trace: %s", raw)
	}
}
