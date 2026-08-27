package observe

import (
	"context"
	"testing"
)

func TestTaskContextFocusRequiresReadAndAllowsExistingSessionFocus(t *testing.T) {
	ctx, collector := WithTaskContextHooks(context.Background(), TaskContextHooks{})
	if _, err := RecordTaskContextFocus(ctx, TaskContextFocusTrace{
		Action: "switch", Certainty: "clear", WorkspaceID: "wp_new",
	}, "", ""); err == nil {
		t.Fatal("selected unread workspace")
	}
	RecordTaskContextExpansion(ctx, TaskContextExpansionTrace{
		Source: "workspace", Operation: "read", ID: "wp_new",
	})
	focus, err := RecordTaskContextFocus(ctx, TaskContextFocusTrace{
		Action: "switch", Certainty: "clear", WorkspaceID: "wp_new",
	}, "", "")
	if err != nil || focus.WorkspaceID != "wp_new" {
		t.Fatalf("valid focus rejected: focus=%+v err=%v", focus, err)
	}
	if _, err := RecordTaskContextFocus(ctx, TaskContextFocusTrace{
		Action: "continue", Certainty: "clear", WorkspaceID: "wp_new",
	}, "wp_new", ""); err == nil {
		t.Fatal("duplicate focus declaration accepted")
	}
	if got := collector.Focus(); got == nil || got.Action != "switch" {
		t.Fatalf("focus not collected: %+v", got)
	}

	ctx2, _ := WithTaskContextHooks(context.Background(), TaskContextHooks{})
	if _, err := RecordTaskContextFocus(ctx2, TaskContextFocusTrace{
		Action: "continue", Certainty: "clear", WorkspaceID: "wp_new",
	}, "wp_new", ""); err != nil {
		t.Fatalf("existing session focus required redundant read: %v", err)
	}
}

func TestTaskContextClarifyAndListTraceAreContentFree(t *testing.T) {
	ctx, collector := WithTaskContextHooks(context.Background(), TaskContextHooks{})
	RecordTaskContextExpansion(ctx, TaskContextExpansionTrace{
		Source: " Workspace ", Operation: " LIST ", ResultIDs: []string{" wp_1 ", "wp_1", "wp_2"},
	})
	focus, err := RecordTaskContextFocus(ctx, TaskContextFocusTrace{
		Action: "clarify", Certainty: "ambiguous", NeedsUserConfirmation: true,
	}, "", "")
	if err != nil || !focus.NeedsUserConfirmation {
		t.Fatalf("clarify rejected: focus=%+v err=%v", focus, err)
	}
	events := collector.Expansions()
	if len(events) != 1 || events[0].Operation != "list" || len(events[0].ResultIDs) != 2 {
		t.Fatalf("list event not normalized: %+v", events)
	}
}

func TestTaskContextFocusRejectsContradictoryShapes(t *testing.T) {
	cases := []TaskContextFocusTrace{
		{Action: "clarify", Certainty: "clear", NeedsUserConfirmation: true},
		{Action: "clarify", Certainty: "ambiguous"},
		{Action: "clear", Certainty: "clear", WorkspaceID: "wp_1"},
		{Action: "switch", Certainty: "ambiguous", WorkspaceID: "wp_1", NeedsUserConfirmation: true},
		{Action: "unknown", Certainty: "clear"},
	}
	for _, event := range cases {
		ctx, _ := WithTaskContextHooks(context.Background(), TaskContextHooks{})
		if _, err := RecordTaskContextFocus(ctx, event, "wp_1", ""); err == nil {
			t.Fatalf("accepted contradictory focus: %+v", event)
		}
	}
}
