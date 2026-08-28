package observe

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"deep-seeing/internal/contextsource"
)

type ContextSourceTrace struct {
	Source  contextsource.Source `json:"source"`
	State   string               `json:"state"`
	Version string               `json:"version,omitempty"`
	Error   string               `json:"error,omitempty"`
}

type ContextCandidateTrace struct {
	Source     contextsource.Source `json:"source"`
	Operation  string               `json:"operation"`
	Query      string               `json:"query,omitempty"`
	ResultIDs  []string             `json:"result_ids,omitempty"`
	Error      string               `json:"error,omitempty"`
	Duration   time.Duration        `json:"duration_ns,omitempty"`
	TurnOffset time.Duration        `json:"turn_offset_ns,omitempty"`
}

type ContextReadTrace struct {
	Source     contextsource.Source `json:"source"`
	ID         string               `json:"id"`
	OK         bool                 `json:"ok"`
	Error      string               `json:"error,omitempty"`
	Duration   time.Duration        `json:"duration_ns,omitempty"`
	TurnOffset time.Duration        `json:"turn_offset_ns,omitempty"`
}

type ContextUseTrace struct {
	Source      contextsource.Source `json:"source"`
	ID          string               `json:"id"`
	Role        contextsource.Role   `json:"role"`
	Disposition string               `json:"disposition"`
	ReasonCode  string               `json:"reason_code,omitempty"`
	TurnOffset  time.Duration        `json:"turn_offset_ns,omitempty"`
}

type ContextHooks struct {
	OnSource    func(ContextSourceTrace)
	OnCandidate func(ContextCandidateTrace)
	OnRead      func(ContextReadTrace)
	OnUse       func(ContextUseTrace)
}

type contextCollectorKey struct{}

type ContextCollector struct {
	mu         sync.Mutex
	sources    []ContextSourceTrace
	candidates []ContextCandidateTrace
	reads      []ContextReadTrace
	uses       []ContextUseTrace
	hooks      ContextHooks
}

func WithContextHooks(ctx context.Context, hooks ContextHooks) (context.Context, *ContextCollector) {
	c := &ContextCollector{hooks: hooks}
	return context.WithValue(ctx, contextCollectorKey{}, c), c
}

func RecordContextSource(ctx context.Context, event ContextSourceTrace) {
	c := contextCollectorFromContext(ctx)
	if c == nil || !contextsource.ValidSource(event.Source) {
		return
	}
	event.State = strings.ToLower(strings.TrimSpace(event.State))
	event.Version = strings.TrimSpace(event.Version)
	event.Error = Preview(event.Error, 160)
	c.mu.Lock()
	c.sources = append(c.sources, event)
	c.mu.Unlock()
	if c.hooks.OnSource != nil {
		c.hooks.OnSource(event)
	}
}

func RecordContextCandidate(ctx context.Context, event ContextCandidateTrace) {
	c := contextCollectorFromContext(ctx)
	if c == nil || !contextsource.ValidSource(event.Source) {
		return
	}
	event.Operation = strings.ToLower(strings.TrimSpace(event.Operation))
	event.Query = Preview(event.Query, 120)
	event.Error = Preview(event.Error, 160)
	var ids []string
	for _, id := range event.ResultIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = appendUniqueString(ids, id)
		}
	}
	event.ResultIDs = ids
	c.mu.Lock()
	c.candidates = append(c.candidates, event)
	c.mu.Unlock()
	if c.hooks.OnCandidate != nil {
		c.hooks.OnCandidate(cloneContextCandidate(event))
	}
}

func RecordContextRead(ctx context.Context, event ContextReadTrace) {
	c := contextCollectorFromContext(ctx)
	if c == nil || !contextsource.ValidSource(event.Source) {
		return
	}
	event.ID = strings.TrimSpace(event.ID)
	event.Error = Preview(event.Error, 160)
	event.OK = event.ID != "" && event.Error == ""
	c.mu.Lock()
	c.reads = append(c.reads, event)
	c.mu.Unlock()
	if c.hooks.OnRead != nil {
		c.hooks.OnRead(event)
	}
}

// RecordContextUse validates the generic public use lifecycle. Domain-specific
// tools may impose stricter rules before calling it.
func RecordContextUse(ctx context.Context, event ContextUseTrace) error {
	c := contextCollectorFromContext(ctx)
	if c == nil {
		return fmt.Errorf("context recorder unavailable")
	}
	event.ID = strings.TrimSpace(event.ID)
	event.Role = contextsource.Role(strings.ToLower(strings.TrimSpace(string(event.Role))))
	event.Disposition = strings.ToLower(strings.TrimSpace(event.Disposition))
	event.ReasonCode = strings.ToLower(strings.TrimSpace(event.ReasonCode))
	if !contextsource.ValidSource(event.Source) || event.ID == "" {
		return fmt.Errorf("valid source and id required")
	}
	if expected := contextsource.RoleFor(event.Source); event.Role == "" {
		event.Role = expected
	} else if event.Role != expected {
		return fmt.Errorf("source %s must use role %s", event.Source, expected)
	}
	if event.Disposition != "used" && event.Disposition != "dismissed" && event.Disposition != "focus" {
		return fmt.Errorf("invalid context disposition %q", event.Disposition)
	}
	if event.Disposition == "dismissed" {
		if !validContextReason(event.ReasonCode) {
			return fmt.Errorf("invalid context reason %q", event.ReasonCode)
		}
	} else if event.ReasonCode != "" {
		return fmt.Errorf("%s context must not include a reason", event.Disposition)
	}

	c.mu.Lock()
	if !c.wasCandidateLocked(event.Source, event.ID) {
		c.mu.Unlock()
		return fmt.Errorf("%s %q was not a candidate in this turn", event.Source, event.ID)
	}
	if event.Disposition == "used" && requiresRead(event.Source) && !c.wasReadLocked(event.Source, event.ID) {
		c.mu.Unlock()
		return fmt.Errorf("used %s %q must be read first", event.Source, event.ID)
	}
	for _, existing := range c.uses {
		if existing.Source == event.Source && existing.ID == event.ID {
			c.mu.Unlock()
			return fmt.Errorf("%s %q already has a context decision", event.Source, event.ID)
		}
	}
	c.uses = append(c.uses, event)
	c.mu.Unlock()
	if c.hooks.OnUse != nil {
		c.hooks.OnUse(event)
	}
	return nil
}

func (c *ContextCollector) Sources() []ContextSourceTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ContextSourceTrace(nil), c.sources...)
}

func (c *ContextCollector) Candidates() []ContextCandidateTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ContextCandidateTrace, len(c.candidates))
	for i, event := range c.candidates {
		out[i] = cloneContextCandidate(event)
	}
	return out
}

func (c *ContextCollector) Reads() []ContextReadTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ContextReadTrace(nil), c.reads...)
}

func (c *ContextCollector) Uses() []ContextUseTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ContextUseTrace(nil), c.uses...)
}

// ContextCandidateKnown exposes only the public candidate lifecycle to tools.
func ContextCandidateKnown(ctx context.Context, source contextsource.Source, id string) bool {
	c := contextCollectorFromContext(ctx)
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wasCandidateLocked(source, strings.TrimSpace(id))
}

// ContextReadKnown exposes only successful public reads to tools.
func ContextReadKnown(ctx context.Context, source contextsource.Source, id string) bool {
	c := contextCollectorFromContext(ctx)
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wasReadLocked(source, strings.TrimSpace(id))
}

func (c *ContextCollector) wasCandidateLocked(source contextsource.Source, id string) bool {
	for _, event := range c.candidates {
		if event.Source != source {
			continue
		}
		for _, candidateID := range event.ResultIDs {
			if candidateID == id {
				return true
			}
		}
	}
	return false
}

func (c *ContextCollector) wasReadLocked(source contextsource.Source, id string) bool {
	for _, event := range c.reads {
		if event.Source == source && event.ID == id && event.OK {
			return true
		}
	}
	return false
}

func contextCollectorFromContext(ctx context.Context) *ContextCollector {
	c, _ := ctx.Value(contextCollectorKey{}).(*ContextCollector)
	return c
}

func cloneContextCandidate(event ContextCandidateTrace) ContextCandidateTrace {
	event.ResultIDs = append([]string(nil), event.ResultIDs...)
	return event
}

func requiresRead(source contextsource.Source) bool {
	return source != contextsource.Bond
}

func validContextReason(reason string) bool {
	switch reason {
	case "irrelevant", "not_applicable", "conflicting", "stale", "insufficient", "superseded", "duplicate", "unconfirmed":
		return true
	default:
		return false
	}
}
