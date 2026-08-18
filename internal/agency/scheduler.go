package agency

import (
	"context"
	"log"
	"sync"
	"time"
)

// Scheduler periodically processes due intents.
type Scheduler struct {
	Runner   *Runner
	AgentID  string
	Interval time.Duration
	Limit    int

	mu     sync.Mutex
	cancel context.CancelFunc
}

// SchedulerSnapshot is the read-only scheduler state for observability.
type SchedulerSnapshot struct {
	Running         bool  `json:"running"`
	IntervalSeconds int64 `json:"interval_seconds"`
	Limit           int   `json:"limit"`
}

// Start begins the ticker loop until Stop or ctx done.
func (s *Scheduler) Start(parent context.Context) {
	if s == nil || s.Runner == nil {
		return
	}
	interval := s.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	limit := s.Limit
	if limit <= 0 {
		limit = 5
	}
	ctx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel
	s.mu.Unlock()

	// Catch-up immediately on boot.
	s.tick(ctx, limit)

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.tick(ctx, limit)
			}
		}
	}()
}

// Stop cancels the scheduler loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// Snapshot reports whether the scheduler loop is active.
func (s *Scheduler) Snapshot() SchedulerSnapshot {
	if s == nil {
		return SchedulerSnapshot{}
	}
	interval := s.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	limit := s.Limit
	if limit <= 0 {
		limit = 5
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return SchedulerSnapshot{
		Running: s.cancel != nil, IntervalSeconds: int64(interval / time.Second), Limit: limit,
	}
}

// TickOnce runs a single due-intent pass (tests / manual).
func (s *Scheduler) TickOnce(ctx context.Context) ([]TurnResult, error) {
	limit := s.Limit
	if limit <= 0 {
		limit = 5
	}
	return s.Runner.ProcessDue(ctx, s.AgentID, limit)
}

func (s *Scheduler) tick(ctx context.Context, limit int) {
	res, err := s.Runner.ProcessDue(ctx, s.AgentID, limit)
	if err != nil {
		log.Printf("agency scheduler: %v", err)
		return
	}
	for _, r := range res {
		if r.Skipped {
			continue
		}
		log.Printf("agency wake intent=%s result=%s session=%s", r.IntentID, r.Result, r.SessionID)
	}
}
