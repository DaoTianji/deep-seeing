package observe

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"deep-seeing/internal/contextsource"
)

// RecallSearchTrace is the observable, non-content result of one episode search.
type RecallSearchTrace struct {
	Query       string   `json:"query,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	ResultCount int      `json:"result_count"`
	ResultIDs   []string `json:"result_ids,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// RecallReadTrace records an explicit request for one Episode body, never the body itself.
type RecallReadTrace struct {
	EpisodeID string `json:"episode_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RecallEvidenceTrace is the Agent's public declaration about a searched candidate.
// Reason is a bounded code, not free-form hidden reasoning.
type RecallEvidenceTrace struct {
	EpisodeID string `json:"episode_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
}

type RecallHooks struct {
	OnSearch   func(RecallSearchTrace)
	OnRead     func(RecallReadTrace)
	OnEvidence func(RecallEvidenceTrace)
}

type recallCollectorKey struct{}

// RecallCollector accumulates tool-level recall events for one turn.
type RecallCollector struct {
	mu         sync.Mutex
	searches   []RecallSearchTrace
	reads      []RecallReadTrace
	evidence   []RecallEvidenceTrace
	onSearch   []func(RecallSearchTrace)
	onRead     func(RecallReadTrace)
	onEvidence func(RecallEvidenceTrace)
}

// WithRecallCollector installs a turn-scoped recall collector.
func WithRecallCollector(ctx context.Context, onSearch ...func(RecallSearchTrace)) (context.Context, *RecallCollector) {
	c := &RecallCollector{onSearch: append([]func(RecallSearchTrace){}, onSearch...)}
	return context.WithValue(ctx, recallCollectorKey{}, c), c
}

// WithRecallHooks installs all public recall lifecycle callbacks for one turn.
func WithRecallHooks(ctx context.Context, hooks RecallHooks) (context.Context, *RecallCollector) {
	c := &RecallCollector{
		onSearch: []func(RecallSearchTrace){hooks.OnSearch},
		onRead:   hooks.OnRead, onEvidence: hooks.OnEvidence,
	}
	return context.WithValue(ctx, recallCollectorKey{}, c), c
}

// RecordRecallSearch appends a search event when the current turn has a collector.
func RecordRecallSearch(ctx context.Context, event RecallSearchTrace) {
	c, _ := ctx.Value(recallCollectorKey{}).(*RecallCollector)
	if c == nil {
		return
	}
	event.Query = Preview(event.Query, 120)
	event.Error = Preview(event.Error, 160)
	event.ResultIDs = append([]string(nil), event.ResultIDs...)
	c.mu.Lock()
	c.searches = append(c.searches, event)
	c.mu.Unlock()
	for _, listener := range c.onSearch {
		if listener != nil {
			listener(cloneRecallSearch(event))
		}
	}
	RecordContextCandidate(ctx, ContextCandidateTrace{
		Source: contextsource.Episode, Operation: "search", Query: event.Query,
		ResultIDs: event.ResultIDs, Error: event.Error,
	})
}

// RecordRecallRead records whether an Episode body was successfully opened.
func RecordRecallRead(ctx context.Context, event RecallReadTrace) {
	c := recallCollectorFromContext(ctx)
	if c == nil {
		return
	}
	event.EpisodeID = strings.TrimSpace(event.EpisodeID)
	event.Error = Preview(event.Error, 160)
	c.mu.Lock()
	c.reads = append(c.reads, event)
	c.mu.Unlock()
	if c.onRead != nil {
		c.onRead(event)
	}
	RecordContextRead(ctx, ContextReadTrace{
		Source: contextsource.Episode, ID: event.EpisodeID, Error: event.Error,
	})
}

// RecordRecallEvidence validates and records public used/dismissed declarations.
func RecordRecallEvidence(ctx context.Context, events []RecallEvidenceTrace) error {
	c := recallCollectorFromContext(ctx)
	if c == nil {
		return fmt.Errorf("recall collector unavailable")
	}
	if len(events) == 0 {
		return fmt.Errorf("at least one evidence decision required")
	}
	c.mu.Lock()
	candidates := map[string]bool{}
	for _, search := range c.searches {
		for _, id := range search.ResultIDs {
			candidates[id] = true
		}
	}
	read := map[string]bool{}
	for _, event := range c.reads {
		if event.EpisodeID != "" && event.Error == "" {
			read[event.EpisodeID] = true
		}
	}
	already := map[string]bool{}
	for _, event := range c.evidence {
		already[event.EpisodeID] = true
	}
	clean := make([]RecallEvidenceTrace, 0, len(events))
	for _, event := range events {
		event.EpisodeID = strings.TrimSpace(event.EpisodeID)
		event.Status = strings.ToLower(strings.TrimSpace(event.Status))
		event.Reason = strings.ToLower(strings.TrimSpace(event.Reason))
		if event.EpisodeID == "" || !candidates[event.EpisodeID] {
			c.mu.Unlock()
			return fmt.Errorf("episode %q was not a candidate in this turn", event.EpisodeID)
		}
		if already[event.EpisodeID] {
			c.mu.Unlock()
			return fmt.Errorf("episode %q already has an evidence decision", event.EpisodeID)
		}
		already[event.EpisodeID] = true
		switch event.Status {
		case "used":
			if !read[event.EpisodeID] {
				c.mu.Unlock()
				return fmt.Errorf("used episode %q must be read first", event.EpisodeID)
			}
			if event.Reason != "" {
				c.mu.Unlock()
				return fmt.Errorf("used episode %q must not include a dismissal reason", event.EpisodeID)
			}
		case "dismissed":
			if !validDismissReason(event.Reason) {
				c.mu.Unlock()
				return fmt.Errorf("invalid dismissal reason %q", event.Reason)
			}
		default:
			c.mu.Unlock()
			return fmt.Errorf("invalid evidence status %q", event.Status)
		}
		clean = append(clean, event)
	}
	c.evidence = append(c.evidence, clean...)
	c.mu.Unlock()
	if c.onEvidence != nil {
		for _, event := range clean {
			c.onEvidence(event)
		}
	}
	for _, event := range clean {
		_ = RecordContextUse(ctx, ContextUseTrace{
			Source: contextsource.Episode, ID: event.EpisodeID,
			Role: contextsource.Evidence, Disposition: event.Status,
			ReasonCode: event.Reason,
		})
	}
	return nil
}

// Searches returns a stable copy of events collected for this turn.
func (c *RecallCollector) Searches() []RecallSearchTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]RecallSearchTrace, len(c.searches))
	for i, event := range c.searches {
		out[i] = cloneRecallSearch(event)
	}
	return out
}

func (c *RecallCollector) Reads() []RecallReadTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RecallReadTrace(nil), c.reads...)
}

func (c *RecallCollector) Evidence() []RecallEvidenceTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RecallEvidenceTrace(nil), c.evidence...)
}

func cloneRecallSearch(event RecallSearchTrace) RecallSearchTrace {
	event.ResultIDs = append([]string(nil), event.ResultIDs...)
	return event
}

func recallCollectorFromContext(ctx context.Context) *RecallCollector {
	c, _ := ctx.Value(recallCollectorKey{}).(*RecallCollector)
	return c
}

func validDismissReason(reason string) bool {
	switch reason {
	case "irrelevant", "conflicting", "stale", "insufficient", "superseded", "duplicate":
		return true
	default:
		return false
	}
}
