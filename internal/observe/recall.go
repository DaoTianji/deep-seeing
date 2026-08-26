package observe

import (
	"context"
	"sync"
)

// RecallSearchTrace is the observable, non-content result of one episode search.
type RecallSearchTrace struct {
	Query       string   `json:"query,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	ResultCount int      `json:"result_count"`
	ResultIDs   []string `json:"result_ids,omitempty"`
	Error       string   `json:"error,omitempty"`
}

type recallCollectorKey struct{}

// RecallCollector accumulates tool-level recall events for one turn.
type RecallCollector struct {
	mu       sync.Mutex
	searches []RecallSearchTrace
	onSearch []func(RecallSearchTrace)
}

// WithRecallCollector installs a turn-scoped recall collector.
func WithRecallCollector(ctx context.Context, onSearch ...func(RecallSearchTrace)) (context.Context, *RecallCollector) {
	c := &RecallCollector{onSearch: append([]func(RecallSearchTrace){}, onSearch...)}
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

func cloneRecallSearch(event RecallSearchTrace) RecallSearchTrace {
	event.ResultIDs = append([]string(nil), event.ResultIDs...)
	return event
}
