package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/workspace"
)

func TestSessionTaskContextFocusStoreLifecycle(t *testing.T) {
	store := NewSessionTaskContextFocusStore()
	store.Apply("s1", observe.TaskContextFocusTrace{Action: "continue", WorkspaceID: "wp_1", IntentID: "int_1"})
	if workspaceID, intentID := store.Current("s1"); workspaceID != "wp_1" || intentID != "int_1" {
		t.Fatalf("unexpected initial focus: %q %q", workspaceID, intentID)
	}
	for _, action := range []string{"check", "compare", "clarify"} {
		store.Apply("s1", observe.TaskContextFocusTrace{Action: action, WorkspaceID: "wp_other"})
	}
	if workspaceID, _ := store.Current("s1"); workspaceID != "wp_1" {
		t.Fatalf("non-selecting action changed focus: %q", workspaceID)
	}
	store.Apply("s1", observe.TaskContextFocusTrace{Action: "switch", WorkspaceID: "wp_2"})
	if workspaceID, intentID := store.Current("s1"); workspaceID != "wp_2" || intentID != "" {
		t.Fatalf("switch did not replace exact focus: %q %q", workspaceID, intentID)
	}
	store.Apply("s1", observe.TaskContextFocusTrace{Action: "clear"})
	if workspaceID, intentID := store.Current("s1"); workspaceID != "" || intentID != "" {
		t.Fatalf("clear retained focus: %q %q", workspaceID, intentID)
	}
}

func TestTaskContextSnapshotPrioritizesActiveSessionFocusOutsideThinLimit(t *testing.T) {
	root := t.TempDir()
	workspaces, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	focusedWorkspace, err := workspaces.Create(workspace.Write{
		Type: workspace.TypeProject, Status: workspace.StatusInProgress,
		Title: "Focused project", Body: "FOCUSED_WORKSPACE_BODY",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := workspaces.Create(workspace.Write{
			Type: workspace.TypeProject, Status: workspace.StatusInProgress,
			Title: "Newer distractor", Body: "DISTRACTOR",
		}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}

	intents, err := intent.OpenStore(filepath.Join(root, "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = intents.Close() })
	scope := identity.LocalCLI()
	base := time.Now().UTC()
	for i := 0; i < 4; i++ {
		if _, err := intents.Create(context.Background(), intent.CreateInput{
			AgentID: scope.AgentID, Kind: intent.IntentOneShot, Title: "Earlier intent",
			DueAt: base.Add(time.Duration(i+1) * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}
	focusedIntent, err := intents.Create(context.Background(), intent.CreateInput{
		AgentID: scope.AgentID, Kind: intent.IntentOneShot, Title: "Focused intent",
		Body: "FOCUSED_INTENT_BODY", DueAt: base.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	focus := NewSessionTaskContextFocusStore()
	focus.Apply("s", observe.TaskContextFocusTrace{
		Action: "continue", WorkspaceID: focusedWorkspace.ID, IntentID: focusedIntent.ID,
	})
	provider := &StoreTaskContextProvider{Workspaces: workspaces, Intents: intents, Focus: focus, Limit: 4}
	snapshot := provider.Snapshot(context.Background(), scope, "s")
	if snapshot.FocusWorkspaceID != focusedWorkspace.ID || snapshot.Workspaces[0].ID != focusedWorkspace.ID {
		t.Fatalf("workspace focus was not prioritized: %+v", snapshot)
	}
	if snapshot.FocusIntentID != focusedIntent.ID || snapshot.Intents[0].ID != focusedIntent.ID {
		t.Fatalf("intent focus was not prioritized: %+v", snapshot)
	}
	if len(snapshot.Workspaces) != 4 || len(snapshot.Intents) != 4 {
		t.Fatalf("focus bypassed thin limits: workspaces=%d intents=%d", len(snapshot.Workspaces), len(snapshot.Intents))
	}
	prompt := snapshot.PromptText()
	if containsAny(prompt, "FOCUSED_WORKSPACE_BODY", "FOCUSED_INTENT_BODY") {
		t.Fatalf("focused bodies leaked into snapshot: %s", prompt)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
