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
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/workspace"
)

func TestContextDiscoveryAndFocusToolLifecycle(t *testing.T) {
	root := t.TempDir()
	workspaces, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	ws, err := workspaces.Create(workspace.Write{
		Type: workspace.TypeProject, Status: workspace.StatusInProgress,
		Title: "Target project", Body: "SECRET_WORKSPACE_BODY",
	})
	if err != nil {
		t.Fatal(err)
	}
	intents, err := intent.OpenStore(filepath.Join(root, "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = intents.Close() })
	it, err := intents.Create(context.Background(), intent.CreateInput{
		AgentID: identity.LocalCLI().AgentID, Kind: intent.IntentOneShot,
		Title: "Target intent", Body: "SECRET_INTENT_BODY", DueAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	focusStore := runtime.NewSessionTaskContextFocusStore()
	all, err := All(Deps{
		Scope: identity.LocalCLI(), Episodes: episodes, Workspace: workspaces, Intents: intents,
		SessionID: "session-1", RecallMode: "agent", TaskContextFocus: focusStore,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, collector := observe.WithTaskContextHooks(context.Background(), observe.TaskContextHooks{})

	intentList, err := findInvokableTool(t, all, "list_intents").InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(intentList, "SECRET_INTENT_BODY") || !strings.Contains(intentList, it.ID) {
		t.Fatalf("intent list leaked body or missed card: %s", intentList)
	}
	workspaceList, err := findInvokableTool(t, all, "list_workspace").InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(workspaceList, ws.ID) {
		t.Fatalf("workspace list missed target: %s", workspaceList)
	}

	invalid, err := findInvokableTool(t, all, "report_context_focus").InvokableRun(ctx,
		`{"action":"switch","certainty":"clear","workspace_id":"`+ws.ID+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(invalid, `"ok":false`) {
		t.Fatalf("unread focus was accepted: %s", invalid)
	}
	if _, err := findInvokableTool(t, all, "read_workspace").InvokableRun(ctx, `{"id":"`+ws.ID+`"}`); err != nil {
		t.Fatal(err)
	}
	accepted, err := findInvokableTool(t, all, "report_context_focus").InvokableRun(ctx,
		`{"action":"switch","certainty":"clear","workspace_id":"`+ws.ID+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(accepted, `"ok":true`) {
		t.Fatalf("read focus was rejected: %s", accepted)
	}
	if workspaceID, intentID := focusStore.Current("session-1"); workspaceID != ws.ID || intentID != "" {
		t.Fatalf("session focus not updated: %q %q", workspaceID, intentID)
	}
	nextSnapshot := runtime.NewStoreTaskContextProvider(workspaces, intents, focusStore).Snapshot(
		context.Background(), identity.LocalCLI(), "session-1",
	)
	if nextSnapshot.FocusWorkspaceID != ws.ID || len(nextSnapshot.Workspaces) == 0 || nextSnapshot.Workspaces[0].ID != ws.ID {
		t.Fatalf("next turn did not receive prioritized focus: %+v", nextSnapshot)
	}
	if strings.Contains(nextSnapshot.PromptText(), "SECRET_WORKSPACE_BODY") {
		t.Fatal("next-turn focused snapshot leaked body")
	}

	continueCtx, continueCollector := observe.WithTaskContextHooks(context.Background(), observe.TaskContextHooks{})
	continued, err := findInvokableTool(t, all, "report_context_focus").InvokableRun(continueCtx,
		`{"action":"continue","certainty":"clear","workspace_id":"`+ws.ID+`"}`)
	if err != nil || !strings.Contains(continued, `"ok":true`) || continueCollector.Focus() == nil {
		t.Fatalf("existing focus could not continue without redundant read: output=%s err=%v", continued, err)
	}
	clearCtx, _ := observe.WithTaskContextHooks(context.Background(), observe.TaskContextHooks{})
	cleared, err := findInvokableTool(t, all, "report_context_focus").InvokableRun(clearCtx,
		`{"action":"clear","certainty":"clear"}`)
	if err != nil || !strings.Contains(cleared, `"ok":true`) {
		t.Fatalf("focus clear failed: output=%s err=%v", cleared, err)
	}
	if workspaceID, intentID := focusStore.Current("session-1"); workspaceID != "" || intentID != "" {
		t.Fatalf("clear retained session focus: %q %q", workspaceID, intentID)
	}

	events := collector.Expansions()
	if len(events) != 3 || events[0].Operation != "list" || events[1].Operation != "list" || events[2].Operation != "read" {
		t.Fatalf("unexpected discovery/read trace: %+v", events)
	}
	trace, err := json.Marshal(observe.TurnTrace{ContextExpands: events, ContextFocus: collector.Focus()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(trace), "SECRET_WORKSPACE_BODY") || strings.Contains(string(trace), "SECRET_INTENT_BODY") {
		t.Fatalf("task context content leaked into trace: %s", trace)
	}
}
