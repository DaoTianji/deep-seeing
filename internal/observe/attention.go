package observe

import (
	"context"
	"strings"
	"sync"
	"time"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
)

// AttentionDecisionTrace is a public working-set transition, not hidden reasoning.
type AttentionDecisionTrace struct {
	Source         contextsource.Source `json:"source"`
	ID             string               `json:"id"`
	From           attention.Tier       `json:"from,omitempty"`
	To             attention.Tier       `json:"to"`
	ReplacedSource contextsource.Source `json:"replaced_source,omitempty"`
	ReplacedID     string               `json:"replaced_id,omitempty"`
	TurnOffset     time.Duration        `json:"turn_offset_ns,omitempty"`
}

type AttentionHooks struct {
	OnSnapshot func(attention.Snapshot)
	OnDecision func(AttentionDecisionTrace)
}

type attentionCollectorKey struct{}

type AttentionCollector struct {
	mu            sync.Mutex
	snapshot      *attention.Snapshot
	finalSnapshot *attention.Snapshot
	decisions     []AttentionDecisionTrace
	hooks         AttentionHooks
	startedAt     time.Time
}

func WithAttentionHooks(ctx context.Context, hooks AttentionHooks) (context.Context, *AttentionCollector) {
	c := &AttentionCollector{hooks: hooks, startedAt: time.Now()}
	return context.WithValue(ctx, attentionCollectorKey{}, c), c
}

func RecordAttentionSnapshot(ctx context.Context, snapshot attention.Snapshot) {
	c := attentionCollectorFromContext(ctx)
	if c == nil {
		return
	}
	copy := cloneAttentionSnapshot(snapshot)
	c.mu.Lock()
	if copy.TurnOffset <= 0 {
		copy.TurnOffset = time.Since(c.startedAt)
	}
	c.snapshot = &copy
	c.mu.Unlock()
	if c.hooks.OnSnapshot != nil {
		c.hooks.OnSnapshot(cloneAttentionSnapshot(copy))
	}
}

// RecordAttentionFinalSnapshot stores the public workspace state after the
// turn. It lets trace replay show recency resets and other end-of-turn changes
// without replacing the snapshot that was visible when the turn started.
func RecordAttentionFinalSnapshot(ctx context.Context, snapshot attention.Snapshot) {
	c := attentionCollectorFromContext(ctx)
	if c == nil {
		return
	}
	copy := cloneAttentionSnapshot(snapshot)
	c.mu.Lock()
	if copy.TurnOffset <= 0 {
		copy.TurnOffset = time.Since(c.startedAt)
	}
	c.finalSnapshot = &copy
	c.mu.Unlock()
	if c.hooks.OnSnapshot != nil {
		c.hooks.OnSnapshot(cloneAttentionSnapshot(copy))
	}
}

func RecordAttentionDecision(ctx context.Context, event AttentionDecisionTrace) {
	c := attentionCollectorFromContext(ctx)
	if c == nil {
		return
	}
	event.ID = strings.TrimSpace(event.ID)
	event.ReplacedID = strings.TrimSpace(event.ReplacedID)
	if !contextsource.ValidSource(event.Source) || event.Source == contextsource.Bond || event.ID == "" {
		return
	}
	c.mu.Lock()
	if event.TurnOffset <= 0 {
		event.TurnOffset = time.Since(c.startedAt)
	}
	c.decisions = append(c.decisions, event)
	c.mu.Unlock()
	if c.hooks.OnDecision != nil {
		c.hooks.OnDecision(event)
	}
}

func (c *AttentionCollector) Snapshot() *attention.Snapshot {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot == nil {
		return nil
	}
	copy := cloneAttentionSnapshot(*c.snapshot)
	return &copy
}

func (c *AttentionCollector) FinalSnapshot() *attention.Snapshot {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.finalSnapshot == nil {
		return nil
	}
	copy := cloneAttentionSnapshot(*c.finalSnapshot)
	return &copy
}

func (c *AttentionCollector) Decisions() []AttentionDecisionTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]AttentionDecisionTrace(nil), c.decisions...)
}

func attentionCollectorFromContext(ctx context.Context) *AttentionCollector {
	c, _ := ctx.Value(attentionCollectorKey{}).(*AttentionCollector)
	return c
}

func cloneAttentionSnapshot(snapshot attention.Snapshot) attention.Snapshot {
	snapshot.Items = append([]attention.Item(nil), snapshot.Items...)
	return snapshot
}
