package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/workspace"
)

type fakeTaskWorkspaceLister struct {
	items []workspace.Document
	err   error
}

func (f fakeTaskWorkspaceLister) List(filter workspace.ListFilter) ([]workspace.Document, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []workspace.Document
	for _, item := range f.items {
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		out = append(out, item)
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

type fakeTaskIntentLister struct {
	items []intent.Intent
	err   error
}

func (f fakeTaskIntentLister) ListActive(context.Context, string, int) ([]intent.Intent, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]intent.Intent(nil), f.items...), nil
}

func TestTaskContextSnapshotIsThinBoundedAndFactsOnly(t *testing.T) {
	now := time.Now().UTC()
	workspaces := fakeTaskWorkspaceLister{items: []workspace.Document{
		{ID: "wp_new", Type: workspace.TypeProject, Status: workspace.StatusInProgress, Title: "T2 task context", Body: "SECRET_WORKSPACE_BODY", Summary: "SECRET_SUMMARY", UpdatedAt: now},
		{ID: "wq_open", Type: workspace.TypeQuestion, Status: workspace.StatusOpen, Title: "Open question", UpdatedAt: now.Add(-time.Minute)},
		{ID: "wp_old", Type: workspace.TypeProject, Status: workspace.StatusOpen, Title: "Older project", UpdatedAt: now.Add(-2 * time.Minute)},
		{ID: "wp_done", Type: workspace.TypeProject, Status: workspace.StatusDone, Title: "Finished project", UpdatedAt: now.Add(time.Minute)},
	}}
	intents := fakeTaskIntentLister{items: []intent.Intent{{
		ID: "in_1", Kind: intent.IntentOneShot, Status: intent.StatusActive,
		Title: "Review T2", Body: "SECRET_INTENT_BODY", DueAt: now.Add(time.Hour),
	}}}
	provider := &StoreTaskContextProvider{Workspaces: workspaces, Intents: intents, Limit: 2}
	snapshot := provider.Snapshot(context.Background(), identity.LocalCLI(), "session-1")

	if len(snapshot.Workspaces) != 2 || snapshot.Workspaces[0].ID != "wp_new" || snapshot.Workspaces[1].ID != "wq_open" {
		t.Fatalf("unexpected workspace cards: %+v", snapshot.Workspaces)
	}
	if len(snapshot.Intents) != 1 || snapshot.Intents[0].ID != "in_1" {
		t.Fatalf("unexpected intent cards: %+v", snapshot.Intents)
	}
	prompt := snapshot.PromptText()
	for _, forbidden := range []string{"SECRET_WORKSPACE_BODY", "SECRET_SUMMARY", "SECRET_INTENT_BODY", "wp_done", "wp_old"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("thin snapshot leaked %q: %s", forbidden, prompt)
		}
	}
	for _, want := range []string{"wp_new", "wq_open", "in_1", "read_workspace", "read_intent", "不是历史证据"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("snapshot missing %q: %s", want, prompt)
		}
	}
	raw, err := json.Marshal(snapshot.Trace())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "T2 task context") || strings.Contains(string(raw), "Review T2") || strings.Contains(string(raw), "SECRET") {
		t.Fatalf("trace contains context content: %s", raw)
	}
}

func TestTaskContextSnapshotDegradesPerSource(t *testing.T) {
	provider := &StoreTaskContextProvider{
		Workspaces: fakeTaskWorkspaceLister{err: errors.New("workspace offline")},
		Intents: fakeTaskIntentLister{items: []intent.Intent{{
			ID: "in_ok", Kind: intent.IntentOneShot, Status: intent.StatusActive, Title: "Still available",
		}}},
	}
	snapshot := provider.Snapshot(context.Background(), identity.LocalCLI(), "s")
	if snapshot.WorkspaceStatus != sourceError || snapshot.IntentStatus != sourceAvailable {
		t.Fatalf("unexpected source status: %+v", snapshot)
	}
	if len(snapshot.Intents) != 1 || len(snapshot.Warnings) == 0 {
		t.Fatalf("partial snapshot lost healthy source or warning: %+v", snapshot)
	}
}

type countingTaskContextProvider struct {
	calls int
}

func (p *countingTaskContextProvider) Snapshot(context.Context, identity.TenantScope, string) TaskContextSnapshot {
	p.calls++
	return TaskContextSnapshot{Version: TaskContextVersion}
}

func TestTaskContextOnlyPreparedInAgentMode(t *testing.T) {
	provider := &countingTaskContextProvider{}
	legacy := &Service{RecallMode: RecallModeLegacy, TaskContext: provider}
	if _, ok := legacy.prepareTaskContext(context.Background()); ok || provider.calls != 0 {
		t.Fatalf("legacy prepared task context: ok=%t calls=%d", ok, provider.calls)
	}
	agent := &Service{Scope: identity.LocalCLI(), SessionID: "s", RecallMode: RecallModeAgent, TaskContext: provider}
	if _, ok := agent.prepareTaskContext(context.Background()); !ok || provider.calls != 1 {
		t.Fatalf("agent did not prepare task context: ok=%t calls=%d", ok, provider.calls)
	}
}
