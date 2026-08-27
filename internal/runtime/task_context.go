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
	TaskContextVersion       = "2"
	defaultTaskContextLimit  = 4
	taskContextTitleMaxRunes = 80
)

type WorkspaceContextLister interface {
	List(workspace.ListFilter) ([]workspace.Document, error)
}

type IntentContextLister interface {
	ListActive(context.Context, string, int) ([]intent.Intent, error)
}

type workspaceContextGetter interface {
	Get(string) (workspace.Document, error)
}

type intentContextGetter interface {
	Get(context.Context, string) (intent.Intent, error)
}

// TaskContextProvider produces an ephemeral, facts-only snapshot for one turn.
type TaskContextProvider interface {
	Snapshot(context.Context, identity.TenantScope, string) TaskContextSnapshot
}

type StoreTaskContextProvider struct {
	Workspaces WorkspaceContextLister
	Intents    IntentContextLister
	Focus      TaskContextFocusReader
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
	Version          string
	PersonID         string
	SessionID        string
	FocusWorkspaceID string
	FocusIntentID    string
	WorkspaceStatus  string
	Workspaces       []TaskWorkspaceCard
	IntentStatus     string
	Intents          []TaskIntentCard
	Warnings         []string
}

func NewStoreTaskContextProvider(workspaces WorkspaceContextLister, intents IntentContextLister, focus ...TaskContextFocusReader) *StoreTaskContextProvider {
	provider := &StoreTaskContextProvider{Workspaces: workspaces, Intents: intents, Limit: defaultTaskContextLimit}
	if len(focus) > 0 {
		provider.Focus = focus[0]
	}
	return provider
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
	focusWorkspaceID, focusIntentID := "", ""
	if p.Focus != nil {
		focusWorkspaceID, focusIntentID = p.Focus.Current(snapshot.SessionID)
	}
	if p.Workspaces != nil {
		snapshot.WorkspaceStatus = sourceAvailable
		snapshot.Workspaces = p.activeWorkspaces(limit, focusWorkspaceID, &snapshot)
		if hasWorkspaceCard(snapshot.Workspaces, focusWorkspaceID) {
			snapshot.FocusWorkspaceID = focusWorkspaceID
		}
	}
	if p.Intents != nil {
		snapshot.IntentStatus = sourceAvailable
		snapshot.Intents = p.activeIntents(ctx, scope.AgentID, limit, focusIntentID, &snapshot)
		if hasIntentCard(snapshot.Intents, focusIntentID) {
			snapshot.FocusIntentID = focusIntentID
		}
	}
	return snapshot
}

func (p *StoreTaskContextProvider) activeWorkspaces(limit int, focusID string, snapshot *TaskContextSnapshot) []TaskWorkspaceCard {
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
	if focusID != "" {
		if _, ok := byID[focusID]; !ok {
			if getter, ok := p.Workspaces.(workspaceContextGetter); ok {
				item, err := getter.Get(focusID)
				if err == nil && activeWorkspaceStatus(item.Status) {
					byID[item.ID] = item
				}
			}
		}
	}
	items := make([]workspace.Document, 0, len(byID))
	for _, item := range byID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == focusID {
			return true
		}
		if items[j].ID == focusID {
			return false
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
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

func (p *StoreTaskContextProvider) activeIntents(ctx context.Context, agentID string, limit int, focusID string, snapshot *TaskContextSnapshot) []TaskIntentCard {
	items, err := p.Intents.ListActive(ctx, agentID, limit)
	if err != nil {
		snapshot.IntentStatus = sourceError
		snapshot.Warnings = append(snapshot.Warnings, "intent: "+observe.Preview(err.Error(), 120))
		return nil
	}
	byID := make(map[string]intent.Intent, len(items)+1)
	for _, item := range items {
		byID[item.ID] = item
	}
	if focusID != "" {
		if _, ok := byID[focusID]; !ok {
			if getter, ok := p.Intents.(intentContextGetter); ok {
				item, err := getter.Get(ctx, focusID)
				if err == nil && item.Status == intent.StatusActive && (agentID == "" || item.AgentID == agentID) {
					byID[item.ID] = item
				}
			}
		}
	}
	items = items[:0]
	for _, item := range byID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == focusID {
			return true
		}
		if items[j].ID == focusID {
			return false
		}
		return items[i].DueAt.Before(items[j].DueAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	cards := make([]TaskIntentCard, 0, len(items))
	for _, item := range items {
		cards = append(cards, TaskIntentCard{
			ID: item.ID, Kind: item.Kind, Status: item.Status,
			Title: trimContextTitle(item.Title), DueAt: item.DueAt,
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
	writeSessionFocus(&b, s)
	writeWorkspaceCards(&b, s)
	writeIntentCards(&b, s)
	b.WriteString("\n会话焦点只是连续性线索，不是正文证据；需要正文时仍应使用 read_workspace / read_intent。目标不在快照时，可用 list_workspace / list_intents 调查。")
	b.WriteString("\n当任务处境实质影响回答时，用 report_context_focus 公开声明最终焦点；如果调查后仍无法可靠区分，用 clarify 声明歧义并询问用户。不要仅凭标题猜正文，用户当前明确表达始终优先。")
	return b.String()
}

func (s TaskContextSnapshot) Trace() observe.TaskContextTrace {
	trace := observe.TaskContextTrace{
		Version: s.Version, FocusWorkspaceID: s.FocusWorkspaceID, FocusIntentID: s.FocusIntentID,
		WorkspaceStatus: s.WorkspaceStatus, IntentStatus: s.IntentStatus,
		Warnings: append([]string(nil), s.Warnings...),
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

func writeSessionFocus(b *strings.Builder, snapshot TaskContextSnapshot) {
	if snapshot.FocusWorkspaceID == "" && snapshot.FocusIntentID == "" {
		b.WriteString("\n- session_focus: none")
		return
	}
	b.WriteString("\n- session_focus:")
	if snapshot.FocusWorkspaceID != "" {
		fmt.Fprintf(b, " workspace=%s", snapshot.FocusWorkspaceID)
	}
	if snapshot.FocusIntentID != "" {
		fmt.Fprintf(b, " intent=%s", snapshot.FocusIntentID)
	}
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
			focus := ""
			if card.ID == snapshot.FocusWorkspaceID {
				focus = " focus=true"
			}
			fmt.Fprintf(b, "\n  - [%s] %s/%s %q updated=%s%s",
				card.ID, card.Type, card.Status, card.Title, card.UpdatedAt.UTC().Format(time.RFC3339), focus)
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
			focus := ""
			if card.ID == snapshot.FocusIntentID {
				focus = " focus=true"
			}
			fmt.Fprintf(b, "\n  - [%s] %s/%s %q due=%s%s",
				card.ID, card.Kind, card.Status, card.Title, card.DueAt.UTC().Format(time.RFC3339), focus)
		}
	case sourceError:
		b.WriteString("\n- intent_leads: temporarily unavailable")
	default:
		b.WriteString("\n- intent_leads: unavailable")
	}
}

func activeWorkspaceStatus(status workspace.Status) bool {
	return status == workspace.StatusOpen || status == workspace.StatusInProgress
}

func hasWorkspaceCard(cards []TaskWorkspaceCard, id string) bool {
	for _, card := range cards {
		if card.ID == id {
			return true
		}
	}
	return false
}

func hasIntentCard(cards []TaskIntentCard, id string) bool {
	for _, card := range cards {
		if card.ID == id {
			return true
		}
	}
	return false
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
