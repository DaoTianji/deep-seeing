package runtime

import (
	"testing"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/observe"
)

func TestSessionAttentionAndFocusDoNotSurviveStoreRestart(t *testing.T) {
	attentionBefore := attention.NewSessionStore(attention.DefaultCapacity())
	if _, err := attentionBefore.Apply("session", []attention.Decision{{
		Source: contextsource.Workspace, ID: "wp_1", Target: attention.Center,
	}}); err != nil {
		t.Fatal(err)
	}
	focusBefore := NewSessionTaskContextFocusStore()
	focusBefore.Apply("session", observe.TaskContextFocusTrace{Action: "continue", WorkspaceID: "wp_1"})

	attentionAfter := attention.NewSessionStore(attention.DefaultCapacity())
	focusAfter := NewSessionTaskContextFocusStore()
	if items := attentionAfter.Snapshot("session").Items; len(items) != 0 {
		t.Fatalf("attention survived process-local store restart: %+v", items)
	}
	if workspaceID, intentID := focusAfter.Current("session"); workspaceID != "" || intentID != "" {
		t.Fatalf("focus survived process-local store restart: %q %q", workspaceID, intentID)
	}
}
