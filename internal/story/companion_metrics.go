package story

import (
	"context"
	"sync"
	"time"

	"deep-seeing/internal/memory"
)

// Metrics are provider reports, not estimates of billing or hidden reasoning.
// Missing usage is represented by fewer ReportedCalls, never by a zero claim.
type CompanionMetrics struct {
	DurationMS    int64             `json:"duration_ms"`
	ModelCalls    int               `json:"model_calls"`
	ReportedCalls int               `json:"reported_calls"`
	Usage         *memory.ChatUsage `json:"usage,omitempty"`
}

type companionMetricsKey struct{}
type companionMeter struct {
	mu              sync.Mutex
	started         time.Time
	calls, reported int
	usage           memory.ChatUsage
}

func newCompanionMeter(ctx context.Context) (context.Context, *companionMeter) {
	m := &companionMeter{started: time.Now()}
	ctx = context.WithValue(ctx, companionMetricsKey{}, m)
	return memory.WithChatUsageObserver(ctx, func(u memory.ChatUsage) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.reported++
		m.usage.PromptTokens += u.PromptTokens
		m.usage.CompletionTokens += u.CompletionTokens
		m.usage.TotalTokens += u.TotalTokens
	}), m
}

func countCompanionCall(ctx context.Context) {
	if m, ok := ctx.Value(companionMetricsKey{}).(*companionMeter); ok {
		m.mu.Lock()
		m.calls++
		m.mu.Unlock()
	}
}

func (m *companionMeter) snapshot() CompanionMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := CompanionMetrics{DurationMS: time.Since(m.started).Milliseconds(), ModelCalls: m.calls, ReportedCalls: m.reported}
	if m.reported > 0 {
		u := m.usage
		out.Usage = &u
	}
	return out
}
