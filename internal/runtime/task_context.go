package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/workspace"
)

const (
	TaskContextVersion       = "1"
	defaultTaskContextLimit  = 4
	taskContextTitleMaxRunes = 80
)

type WorkspaceContextLister interface {
	List(workspace.ListFilter) ([]workspace.Document, error)
}

type IntentContextLister interface {
	ListActive(context.Context, string, int) ([]intent.Intent, error)
}

// TaskContextProvider produces an ephemeral, facts-only snapshot for one turn.
type TaskContextProvider interface {
	Snapshot(context.Context, identity.TenantScope, string) TaskContextSnapshot
}

type StoreTaskContextProvider struct {
	Workspaces WorkspaceContextLister
	Intents    IntentContextLister
	Limit      int
}

type TaskWorkspaceCard struct {
	ID        string
	Type      workspace.Type
	Status    workspace.Status
	Title     string
	UpdatedAt time.Time
}

type TaskIntentCard struct {
	ID     string
	Kind   intent.IntentKind
	Status intent.Status
	Title  string
	DueAt  time.Time
}

// TaskContextSnapshot is a bounded view of current task/project facts.
// It is rebuilt per turn and is never written to LTM.
type TaskContextSnapshot struct {
	Version         string
	PersonID        string
	SessionID       string
	WorkspaceStatus string
	Workspaces      []TaskWorkspaceCard
	IntentStatus    string
	Intents         []TaskIntentCard
	Warnings        []string
}

func NewStoreTaskContextProvider(workspaces WorkspaceContextLister, intents IntentContextLister) *StoreTaskContextProvider {
	return &StoreTaskContextProvider{Workspaces: workspaces, Intents: intents, Limit: defaultTaskContextLimit}
}

func (p *StoreTaskContextProvider) Snapshot(ctx context.Context, scope identity.TenantScope, sessionID string) TaskContextSnapshot {
	snapshot := TaskContextSnapshot{
		Version: TaskContextVersion, PersonID: scope.PersonID(), SessionID: strings.TrimSpace(sessionID),
		WorkspaceStatus: sourceUnavailable, IntentStatus: sourceUnavailable,
	}
	if p == nil {
		return snapshot
	}
	limit := p.Limit
	if limit <= 0 {
		limit = defaultTaskContextLimit
	}
	if p.Workspaces != nil {
		snapshot.WorkspaceStatus = sourceAvailable
		snapshot.Workspaces = p.activeWorkspaces(limit, &snapshot)
	}
	if p.Intents != nil {
		snapshot.IntentStatus = sourceAvailable
		items, err := p.Intents.ListActive(ctx, scope.AgentID, limit)
		if err != nil {
			snapshot.IntentStatus = sourceError
			snapshot.Warnings = append(snapshot.Warnings, "intent: "+observe.Preview(err.Error(), 120))
		} else {
			if len(items) > limit {
				items = items[:limit]
			}
			for _, item := range items {
				snapshot.Intents = append(snapshot.Intents, TaskIntentCard{
					ID: item.ID, Kind: item.Kind, Status: item.Status,
					Title: trimContextTitle(item.Title), DueAt: item.DueAt,
				})
			}
		}
	}
	return snapshot
}

func (p *StoreTaskContextProvider) activeWorkspaces(limit int, snapshot *TaskContextSnapshot) []TaskWorkspaceCard {
	byID := map[string]workspace.Document{}
	for _, status := range []workspace.Status{workspace.StatusOpen, workspace.StatusInProgress} {
		items, err := p.Workspaces.List(workspace.ListFilter{Status: status, Limit: limit})
		if err != nil {
			snapshot.WorkspaceStatus = sourceError
			snapshot.Warnings = append(snapshot.Warnings, "workspace: "+observe.Preview(err.Error(), 120))
			continue
		}
		for _, item := range items {
			byID[item.ID] = item
		}
	}
	items := make([]workspace.Document, 0, len(byID))
	for _, item := range byID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	if len(items) > limit {
		items = items[:limit]
	}
	cards := make([]TaskWorkspaceCard, 0, len(items))
	for _, item := range items {
		cards = append(cards, TaskWorkspaceCard{
			ID: item.ID, Type: item.Type, Status: item.Status,
			Title: trimContextTitle(item.Title), UpdatedAt: item.UpdatedAt,
		})
	}
	return cards
}

func (s TaskContextSnapshot) PromptText() string {
	var b strings.Builder
	b.WriteString("这是每轮重建的任务/项目事实快照，不是历史证据，也不表示这些条目与本轮必然相关。")
	if s.PersonID != "" {
		fmt.Fprintf(&b, "\n- current_person: %s", s.PersonID)
	}
	if s.SessionID != "" {
		fmt.Fprintf(&b, "\n- session: %s", s.SessionID)
	}
	writeWorkspaceCards(&b, s)
	writeIntentCards(&b, s)
	b.WriteString("\n需要延续某项工作或未来约定时，按 ID 主动调用 read_workspace / read_intent 展开；不要仅凭标题推断正文。当前用户消息和 STM 已在对话中，无需从快照重复推断。")
	return b.String()
}

func (s TaskContextSnapshot) Trace() observe.TaskContextTrace {
	trace := observe.TaskContextTrace{
		Version: s.Version, WorkspaceStatus: s.WorkspaceStatus,
		IntentStatus: s.IntentStatus, Warnings: append([]string(nil), s.Warnings...),
	}
	for _, card := range s.Workspaces {
		trace.WorkspaceIDs = append(trace.WorkspaceIDs, card.ID)
	}
	for _, card := range s.Intents {
		trace.IntentIDs = append(trace.IntentIDs, card.ID)
	}
	return trace
}

func (s *Service) prepareTaskContext(ctx context.Context) (TaskContextSnapshot, bool) {
	if s == nil || s.RecallMode != RecallModeAgent || s.TaskContext == nil {
		return TaskContextSnapshot{}, false
	}
	return s.TaskContext.Snapshot(ctx, s.Scope, s.SessionID), true
}

func writeWorkspaceCards(b *strings.Builder, snapshot TaskContextSnapshot) {
	switch snapshot.WorkspaceStatus {
	case sourceAvailable:
		if len(snapshot.Workspaces) == 0 {
			b.WriteString("\n- workspace_leads: none")
			return
		}
		b.WriteString("\n- workspace_leads:")
		for _, card := range snapshot.Workspaces {
			fmt.Fprintf(b, "\n  - [%s] %s/%s %q updated=%s",
				card.ID, card.Type, card.Status, card.Title, card.UpdatedAt.UTC().Format(time.RFC3339))
		}
	case sourceError:
		b.WriteString("\n- workspace_leads: temporarily unavailable")
	default:
		b.WriteString("\n- workspace_leads: unavailable")
	}
}

func writeIntentCards(b *strings.Builder, snapshot TaskContextSnapshot) {
	switch snapshot.IntentStatus {
	case sourceAvailable:
		if len(snapshot.Intents) == 0 {
			b.WriteString("\n- intent_leads: none")
			return
		}
		b.WriteString("\n- intent_leads:")
		for _, card := range snapshot.Intents {
			fmt.Fprintf(b, "\n  - [%s] %s/%s %q due=%s",
				card.ID, card.Kind, card.Status, card.Title, card.DueAt.UTC().Format(time.RFC3339))
		}
	case sourceError:
		b.WriteString("\n- intent_leads: temporarily unavailable")
	default:
		b.WriteString("\n- intent_leads: unavailable")
	}
}

func trimContextTitle(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= taskContextTitleMaxRunes {
		return value
	}
	return string(runes[:taskContextTitleMaxRunes]) + "…"
}

const (
	sourceAvailable   = "available"
	sourceUnavailable = "unavailable"
	sourceError       = "error"
)
