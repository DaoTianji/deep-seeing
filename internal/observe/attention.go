package observe

import (
	"context"
	"strings"
	"sync"

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
}

type AttentionHooks struct {
	OnSnapshot func(attention.Snapshot)
	OnDecision func(AttentionDecisionTrace)
}

type attentionCollectorKey struct{}

type AttentionCollector struct {
	mu        sync.Mutex
	snapshot  *attention.Snapshot
	decisions []AttentionDecisionTrace
	hooks     AttentionHooks
}

func WithAttentionHooks(ctx context.Context, hooks AttentionHooks) (context.Context, *AttentionCollector) {
	c := &AttentionCollector{hooks: hooks}
	return context.WithValue(ctx, attentionCollectorKey{}, c), c
}

func RecordAttentionSnapshot(ctx context.Context, snapshot attention.Snapshot) {
	c := attentionCollectorFromContext(ctx)
	if c == nil {
		return
	}
	copy := cloneAttentionSnapshot(snapshot)
	c.mu.Lock()
	c.snapshot = &copy
	c.mu.Unlock()
	if c.hooks.OnSnapshot != nil {
		c.hooks.OnSnapshot(cloneAttentionSnapshot(snapshot))
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
