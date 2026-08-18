package agency

import (
	"context"
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/intent"
	"deep-seeing/internal/runtime"
)

// TurnResult is the outcome of one autonomous wake.
type TurnResult struct {
	IntentID  string `json:"intent_id"`
	WakeJobID string `json:"wake_job_id"`
	SessionID string `json:"session_id"`
	Decision  string `json:"decision"`
	Result    string `json:"result"` // sleep|noop|acted|cancelled|skipped
	Notes     string `json:"notes"`
	Skipped   bool   `json:"skipped,omitempty"`
}

// TurnHandler optionally runs a real agent turn. Nil → noop/sleep only.
type TurnHandler func(ctx context.Context, sessionID string, it intent.Intent, decision intent.CatchUpDecision) (acted bool, notes string, err error)

// Runner executes due intents through ExecutionQueue.
type Runner struct {
	Store   *intent.Store
	Queue   *runtime.ExecutionQueue
	Budget  *Budget
	Handler TurnHandler // optional
	Now     func() time.Time
}

func (r *Runner) now() time.Time {
	if r != nil && r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// ProcessDue wakes due intents (catch-up merged). Returns per-intent results.
func (r *Runner) ProcessDue(ctx context.Context, agentID string, limit int) ([]TurnResult, error) {
	if r == nil || r.Store == nil {
		return nil, fmt.Errorf("agency runner incomplete")
	}
	if r.Queue == nil {
		r.Queue = runtime.NewExecutionQueue(agentID)
	}
	if r.Budget == nil {
		r.Budget = DefaultBudget()
	}
	now := r.now()
	due, err := r.Store.ListDue(ctx, agentID, now, limit)
	if err != nil {
		return nil, err
	}
	var out []TurnResult
	for _, it := range due {
		res, err := r.wakeOne(ctx, it, now)
		if err != nil {
			return out, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (r *Runner) wakeOne(ctx context.Context, it intent.Intent, now time.Time) (TurnResult, error) {
	missed := intent.EstimateMissedTicks(it, now)
	decision := intent.DecideCatchUp(intent.CatchUpInput{
		Kind: it.Kind, MissedTicks: missed, DueAt: it.DueAt, Now: now,
		StaleAfterDays: it.StaleAfterDays,
	})

	if decision.Action == intent.CatchUpContinue && !decision.Overdue {
		return TurnResult{IntentID: it.ID, Decision: string(decision.Action), Result: "skipped", Skipped: true, Notes: decision.Reason}, nil
	}
	if decision.Action == intent.CatchUpCancel {
		_, _ = r.Store.SetStatus(ctx, it.ID, intent.StatusCancelled)
		job, _ := r.Store.InsertWakeJob(ctx, intent.WakeJob{
			IntentID: it.ID, AgentID: it.AgentID, Trigger: it.Trigger,
			ScheduledAt: now, StartedAt: now, Status: "done",
			Decision: string(decision.Action), Result: "cancelled", Notes: decision.Reason,
		})
		return TurnResult{IntentID: it.ID, WakeJobID: job.ID, Decision: string(decision.Action), Result: "cancelled", Notes: decision.Reason}, nil
	}

	if ok, why := r.Budget.AllowWake(now); !ok {
		job, _ := r.Store.InsertWakeJob(ctx, intent.WakeJob{
			IntentID: it.ID, AgentID: it.AgentID, Trigger: it.Trigger,
			ScheduledAt: now, Status: "skipped", Decision: string(decision.Action),
			Result: "skipped", Notes: why,
		})
		return TurnResult{IntentID: it.ID, WakeJobID: job.ID, Decision: string(decision.Action), Result: "skipped", Skipped: true, Notes: why}, nil
	}

	// Hard rule: outbound contact requires both budget flag and intent flag.
	if it.AllowOutbound && !r.Budget.OutboundAllowed() {
		// still allow internal wake; just note outbound suppressed
		decision.Reason = strings.TrimSpace(decision.Reason + "; outbound contact suppressed by default budget")
	}

	attempt := it.Attempt + 1
	sessionID := runtime.AutoSessionID(it.ID, attempt)
	job, err := r.Store.InsertWakeJob(ctx, intent.WakeJob{
		IntentID: it.ID, AgentID: it.AgentID, Trigger: it.Trigger,
		ScheduledAt: now, StartedAt: now, Status: "running",
		Decision: string(decision.Action), SessionID: sessionID,
		Notes: decision.Reason,
	})
	if err != nil {
		return TurnResult{}, err
	}

	r.Budget.ConsumeWake(now)

	var result = "noop"
	var notes = decision.Reason
	runErr := r.Queue.RunCognitive(ctx, "auto", func(turnCtx context.Context) error {
		if r.Handler == nil {
			result = "noop"
			notes = "autonomous turn skeleton: noop (no LLM handler); intent acknowledged"
			return nil
		}
		acted, hNotes, err := r.Handler(turnCtx, sessionID, it, decision)
		if err != nil {
			return err
		}
		if acted {
			result = "acted"
		} else {
			result = "noop"
		}
		if strings.TrimSpace(hNotes) != "" {
			notes = hNotes
		}
		return nil
	})
	if runErr != nil {
		_ = r.Store.FinishWakeJob(ctx, job.ID, "failed", string(decision.Action), "failed", runErr.Error())
		return TurnResult{IntentID: it.ID, WakeJobID: job.ID, SessionID: sessionID, Decision: string(decision.Action), Result: "failed", Notes: runErr.Error()}, runErr
	}

	// Advance schedule
	nextStatus := intent.StatusActive
	var nextDue time.Time
	switch it.Kind {
	case intent.IntentOneShot:
		nextStatus = intent.StatusDone
		nextDue = it.DueAt
	default:
		if it.Interval > 0 {
			nextDue = now.Add(it.Interval)
		} else {
			nextDue = now.Add(24 * time.Hour)
		}
	}
	_, _ = r.Store.MarkWoke(ctx, it.ID, now, nextDue, nextStatus)
	_ = r.Store.FinishWakeJob(ctx, job.ID, "done", string(decision.Action), result, notes)

	return TurnResult{
		IntentID: it.ID, WakeJobID: job.ID, SessionID: sessionID,
		Decision: string(decision.Action), Result: result, Notes: notes,
	}, nil
}
