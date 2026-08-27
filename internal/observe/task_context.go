package observe

import (
	"context"
	"strings"
	"sync"
)

// TaskContextTrace records the bounded factual leads presented to the Agent.
// It contains identifiers and source health only, never Workspace or Intent bodies.
type TaskContextTrace struct {
	Version         string   `json:"version"`
	WorkspaceStatus string   `json:"workspace_status,omitempty"`
	WorkspaceIDs    []string `json:"workspace_ids,omitempty"`
	IntentStatus    string   `json:"intent_status,omitempty"`
	IntentIDs       []string `json:"intent_ids,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

// TaskContextExpansionTrace records an explicit read of one task-context lead.
type TaskContextExpansionTrace struct {
	Source string `json:"source"`
	ID     string `json:"id,omitempty"`
	Error  string `json:"error,omitempty"`
}

type TaskContextHooks struct {
	OnExpand func(TaskContextExpansionTrace)
}

type taskContextCollectorKey struct{}

// TaskContextCollector accumulates explicit context expansion events for one turn.
type TaskContextCollector struct {
	mu       sync.Mutex
	expands  []TaskContextExpansionTrace
	onExpand func(TaskContextExpansionTrace)
}

func WithTaskContextHooks(ctx context.Context, hooks TaskContextHooks) (context.Context, *TaskContextCollector) {
	c := &TaskContextCollector{onExpand: hooks.OnExpand}
	return context.WithValue(ctx, taskContextCollectorKey{}, c), c
}

// RecordTaskContextExpansion records a public tool event without storing returned content.
func RecordTaskContextExpansion(ctx context.Context, event TaskContextExpansionTrace) {
	c, _ := ctx.Value(taskContextCollectorKey{}).(*TaskContextCollector)
	if c == nil {
		return
	}
	event.Source = strings.ToLower(strings.TrimSpace(event.Source))
	event.ID = strings.TrimSpace(event.ID)
	event.Error = Preview(event.Error, 160)
	c.mu.Lock()
	c.expands = append(c.expands, event)
	c.mu.Unlock()
	if c.onExpand != nil {
		c.onExpand(event)
	}
}

func (c *TaskContextCollector) Expansions() []TaskContextExpansionTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]TaskContextExpansionTrace(nil), c.expands...)
}
