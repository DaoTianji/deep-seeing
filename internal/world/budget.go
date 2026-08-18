package world

import (
	"sync"
	"time"
)

// FetchBudget limits remote fetches per UTC day.
type FetchBudget struct {
	mu        sync.Mutex
	MaxPerDay int
	day       string
	used      int
}

// DefaultFetchBudget is conservative.
func DefaultFetchBudget() *FetchBudget {
	return &FetchBudget{MaxPerDay: 40}
}

// Allow reports whether another fetch is within budget.
func (b *FetchBudget) Allow(now time.Time) (bool, string) {
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
		max = 40
	}
	if b.used >= max {
		return false, "daily world-fetch budget exhausted"
	}
	return true, ""
}

// Consume records a fetch.
func (b *FetchBudget) Consume(now time.Time) {
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
