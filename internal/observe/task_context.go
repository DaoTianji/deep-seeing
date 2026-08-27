package observe

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"deep-seeing/internal/contextsource"
)

// TaskContextTrace records the bounded factual leads presented to the Agent.
// It contains identifiers and source health only, never Workspace or Intent bodies.
type TaskContextTrace struct {
	Version          string   `json:"version"`
	FocusWorkspaceID string   `json:"focus_workspace_id,omitempty"`
	FocusIntentID    string   `json:"focus_intent_id,omitempty"`
	WorkspaceStatus  string   `json:"workspace_status,omitempty"`
	WorkspaceIDs     []string `json:"workspace_ids,omitempty"`
	IntentStatus     string   `json:"intent_status,omitempty"`
	IntentIDs        []string `json:"intent_ids,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

// TaskContextExpansionTrace records public discovery/read activity without content.
type TaskContextExpansionTrace struct {
	Source    string   `json:"source"`
	Operation string   `json:"operation,omitempty"`
	ID        string   `json:"id,omitempty"`
	ResultIDs []string `json:"result_ids,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// TaskContextFocusTrace is a public, structured context conclusion, not hidden reasoning.
type TaskContextFocusTrace struct {
	Action                string `json:"action"`
	Certainty             string `json:"certainty"`
	WorkspaceID           string `json:"workspace_id,omitempty"`
	IntentID              string `json:"intent_id,omitempty"`
	NeedsUserConfirmation bool   `json:"needs_user_confirmation,omitempty"`
}

type TaskContextHooks struct {
	OnExpand func(TaskContextExpansionTrace)
	OnFocus  func(TaskContextFocusTrace)
}

type taskContextCollectorKey struct{}

// TaskContextCollector accumulates public task-context events for one turn.
type TaskContextCollector struct {
	mu       sync.Mutex
	expands  []TaskContextExpansionTrace
	focus    *TaskContextFocusTrace
	onExpand func(TaskContextExpansionTrace)
	onFocus  func(TaskContextFocusTrace)
}

func WithTaskContextHooks(ctx context.Context, hooks TaskContextHooks) (context.Context, *TaskContextCollector) {
	c := &TaskContextCollector{onExpand: hooks.OnExpand, onFocus: hooks.OnFocus}
	return context.WithValue(ctx, taskContextCollectorKey{}, c), c
}

// RecordTaskContextExpansion records a public tool event without storing returned content.
func RecordTaskContextExpansion(ctx context.Context, event TaskContextExpansionTrace) {
	c, _ := ctx.Value(taskContextCollectorKey{}).(*TaskContextCollector)
	if c == nil {
		return
	}
	event.Source = strings.ToLower(strings.TrimSpace(event.Source))
	event.Operation = strings.ToLower(strings.TrimSpace(event.Operation))
	if event.Operation == "" {
		event.Operation = "read"
	}
	event.ID = strings.TrimSpace(event.ID)
	var resultIDs []string
	for _, id := range event.ResultIDs {
		if id = strings.TrimSpace(id); id != "" {
			resultIDs = appendUniqueString(resultIDs, id)
		}
	}
	event.ResultIDs = resultIDs
	event.Error = Preview(event.Error, 160)
	c.mu.Lock()
	c.expands = append(c.expands, event)
	c.mu.Unlock()
	if c.onExpand != nil {
		c.onExpand(event)
	}
	source := contextsource.Source(event.Source)
	if event.Operation == "list" {
		RecordContextCandidate(ctx, ContextCandidateTrace{
			Source: source, Operation: "list", ResultIDs: event.ResultIDs, Error: event.Error,
		})
	} else {
		RecordContextRead(ctx, ContextReadTrace{Source: source, ID: event.ID, Error: event.Error})
	}
}

// RecordTaskContextFocus validates and records one public focus conclusion.
// A newly selected ID must have been read successfully in this turn; an existing
// session focus may be continued without a redundant read.
func RecordTaskContextFocus(ctx context.Context, event TaskContextFocusTrace, currentWorkspaceID, currentIntentID string) (TaskContextFocusTrace, error) {
	c, _ := ctx.Value(taskContextCollectorKey{}).(*TaskContextCollector)
	if c == nil {
		return TaskContextFocusTrace{}, fmt.Errorf("task context recorder unavailable")
	}
	event.Action = strings.ToLower(strings.TrimSpace(event.Action))
	event.Certainty = strings.ToLower(strings.TrimSpace(event.Certainty))
	event.WorkspaceID = strings.TrimSpace(event.WorkspaceID)
	event.IntentID = strings.TrimSpace(event.IntentID)

	c.mu.Lock()
	if c.focus != nil {
		c.mu.Unlock()
		return TaskContextFocusTrace{}, fmt.Errorf("context focus already reported this turn")
	}
	if err := c.validateFocusLocked(event, strings.TrimSpace(currentWorkspaceID), strings.TrimSpace(currentIntentID)); err != nil {
		c.mu.Unlock()
		return TaskContextFocusTrace{}, err
	}
	copy := event
	c.focus = &copy
	c.mu.Unlock()
	if c.onFocus != nil {
		c.onFocus(copy)
	}
	disposition := "used"
	if copy.Action == "continue" || copy.Action == "switch" {
		disposition = "focus"
	}
	if copy.WorkspaceID != "" {
		_ = RecordContextUse(ctx, ContextUseTrace{
			Source: contextsource.Workspace, ID: copy.WorkspaceID,
			Role: contextsource.Task, Disposition: disposition,
		})
	}
	if copy.IntentID != "" {
		_ = RecordContextUse(ctx, ContextUseTrace{
			Source: contextsource.Intent, ID: copy.IntentID,
			Role: contextsource.Plan, Disposition: disposition,
		})
	}
	return copy, nil
}

func (c *TaskContextCollector) validateFocusLocked(event TaskContextFocusTrace, currentWorkspaceID, currentIntentID string) error {
	switch event.Action {
	case "clear":
		if event.Certainty != "clear" || event.WorkspaceID != "" || event.IntentID != "" || event.NeedsUserConfirmation {
			return fmt.Errorf("clear requires certainty=clear, no IDs, and no confirmation")
		}
		return nil
	case "clarify":
		if event.Certainty != "ambiguous" || !event.NeedsUserConfirmation || event.WorkspaceID != "" || event.IntentID != "" {
			return fmt.Errorf("clarify requires certainty=ambiguous, confirmation=true, and no selected IDs")
		}
		return nil
	case "continue", "switch", "check", "compare":
		if event.Certainty != "clear" || event.NeedsUserConfirmation {
			return fmt.Errorf("%s requires certainty=clear and no confirmation", event.Action)
		}
		if event.WorkspaceID == "" && event.IntentID == "" {
			return fmt.Errorf("%s requires a workspace_id or intent_id", event.Action)
		}
	default:
		return fmt.Errorf("unsupported context action %q", event.Action)
	}
	if event.WorkspaceID != "" && event.WorkspaceID != currentWorkspaceID && !c.wasReadLocked("workspace", event.WorkspaceID) {
		return fmt.Errorf("workspace_id must be read this turn before selection")
	}
	if event.IntentID != "" && event.IntentID != currentIntentID && !c.wasReadLocked("intent", event.IntentID) {
		return fmt.Errorf("intent_id must be read this turn before selection")
	}
	return nil
}

func (c *TaskContextCollector) wasReadLocked(source, id string) bool {
	for _, event := range c.expands {
		if event.Source == source && event.Operation == "read" && event.ID == id && event.Error == "" {
			return true
		}
	}
	return false
}

func (c *TaskContextCollector) Expansions() []TaskContextExpansionTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]TaskContextExpansionTrace(nil), c.expands...)
}

func (c *TaskContextCollector) Focus() *TaskContextFocusTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.focus == nil {
		return nil
	}
	copy := *c.focus
	return &copy
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
