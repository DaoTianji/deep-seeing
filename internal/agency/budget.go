package agency

import (
	"sync"
	"time"
)

// Budget limits autonomous wakes (not chat turns).
type Budget struct {
	mu            sync.Mutex
	MaxPerDay     int
	AllowOutbound bool // default false — never proactive contact unless enabled
	day           string
	used          int
}

// BudgetSnapshot is the read-only autonomous wake budget view.
type BudgetSnapshot struct {
	Day           string `json:"day"`
	Used          int    `json:"used"`
	MaxPerDay     int    `json:"max_per_day"`
	Remaining     int    `json:"remaining"`
	AllowOutbound bool   `json:"allow_outbound"`
}

// DefaultBudget is conservative: few wakes/day, no outbound.
func DefaultBudget() *Budget {
	return &Budget{MaxPerDay: 8, AllowOutbound: false}
}

// AllowWake reports whether another autonomous wake is within budget.
func (b *Budget) AllowWake(now time.Time) (bool, string) {
	if b == nil {
		return true, ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	day := now.UTC().Format("2006-01-02")
	if b.day != day {
		b.day = day
		b.used = 0
	}
	max := b.MaxPerDay
	if max <= 0 {
		max = 8
	}
	if b.used >= max {
		return false, "daily autonomous budget exhausted"
	}
	return true, ""
}

// ConsumeWake records a wake attempt.
func (b *Budget) ConsumeWake(now time.Time) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	day := now.UTC().Format("2006-01-02")
	if b.day != day {
		b.day = day
		b.used = 0
	}
	b.used++
}

// OutboundAllowed is the hard default for proactive human contact.
func (b *Budget) OutboundAllowed() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.AllowOutbound
}

// Snapshot returns today's autonomous wake budget without consuming it.
func (b *Budget) Snapshot(now time.Time) BudgetSnapshot {
	if b == nil {
		return BudgetSnapshot{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	day := now.UTC().Format("2006-01-02")
	if b.day != day {
		b.day = day
		b.used = 0
	}
	max := b.MaxPerDay
	if max <= 0 {
		max = 8
	}
	remaining := max - b.used
	if remaining < 0 {
		remaining = 0
	}
	return BudgetSnapshot{
		Day: day, Used: b.used, MaxPerDay: max, Remaining: remaining,
		AllowOutbound: b.AllowOutbound,
	}
}
