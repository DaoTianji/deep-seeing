// Package intent holds P7 Intent catch-up / stale contracts (scheduler not implemented yet).
package intent

import (
	"strings"
	"time"
)

// CatchUpAction is how a wake should treat overdue Intent ticks.
type CatchUpAction string

const (
	CatchUpContinue   CatchUpAction = "continue"
	CatchUpReschedule CatchUpAction = "reschedule"
	CatchUpCancel     CatchUpAction = "cancel"
	CatchUpDoNow      CatchUpAction = "do_now"
)

// IntentKind distinguishes recurring vs one-shot intents.
type IntentKind string

const (
	IntentRecurring IntentKind = "recurring"
	IntentOneShot   IntentKind = "one_shot"
)

// CatchUpInput describes overdue wake facts for a policy decision.
type CatchUpInput struct {
	Kind           IntentKind
	MissedTicks    int
	DueAt          time.Time
	Now            time.Time
	StaleAfterDays int // one-shot: days past due before stale; 0 → default 7
}

// CatchUpDecision is the P5.0 catch-up / stale contract output.
type CatchUpDecision struct {
	Action      CatchUpAction `json:"action"`
	MissedTicks int           `json:"missed_ticks,omitempty"`
	StaleDays   int           `json:"stale_days,omitempty"`
	MergedWake  bool          `json:"merged_wake,omitempty"` // true: wake once for many missed ticks
	Reason      string        `json:"reason"`
	Overdue     bool          `json:"overdue"`
}

// DecideCatchUp merges missed ticks for recurring intents and surfaces stale one-shots.
// No Scheduler is invoked — this is the decision shape only.
func DecideCatchUp(in CatchUpInput) CatchUpDecision {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	due := in.DueAt
	if due.IsZero() {
		return CatchUpDecision{Action: CatchUpContinue, Reason: "no due_at; continue"}
	}
	if !now.After(due) {
		return CatchUpDecision{Action: CatchUpContinue, Reason: "not due yet"}
	}

	staleDays := int(now.Sub(due).Hours() / 24)
	if staleDays < 0 {
		staleDays = 0
	}
	kind := in.Kind
	if kind == "" {
		kind = IntentRecurring
	}

	switch kind {
	case IntentOneShot:
		limit := in.StaleAfterDays
		if limit <= 0 {
			limit = 7
		}
		if staleDays >= limit {
			return CatchUpDecision{
				Action: CatchUpCancel, StaleDays: staleDays, Overdue: true,
				Reason: "one_shot intent stale; do not pretend it fired on time",
			}
		}
		return CatchUpDecision{
			Action: CatchUpDoNow, StaleDays: staleDays, Overdue: true,
			Reason: "one_shot overdue but within stale window; do_now with overdue context",
		}
	default:
		missed := in.MissedTicks
		if missed < 1 {
			missed = 1
		}
		return CatchUpDecision{
			Action: CatchUpDoNow, MissedTicks: missed, StaleDays: staleDays,
			MergedWake: true, Overdue: true,
			Reason: "merge missed ticks into one wake; explain overdue to agent",
		}
	}
}

// NormalizeCatchUpAction maps raw action strings.
func NormalizeCatchUpAction(raw string) CatchUpAction {
	switch CatchUpAction(strings.ToLower(strings.TrimSpace(raw))) {
	case CatchUpReschedule:
		return CatchUpReschedule
	case CatchUpCancel:
		return CatchUpCancel
	case CatchUpDoNow:
		return CatchUpDoNow
	default:
		return CatchUpContinue
	}
}
