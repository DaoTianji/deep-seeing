package intent_test

import (
	"testing"
	"time"

	"deep-seeing/internal/intent"
)

func TestRecurringMergesMissedTicks(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	due := now.Add(-72 * time.Hour)
	d := intent.DecideCatchUp(intent.CatchUpInput{
		Kind: intent.IntentRecurring, MissedTicks: 3, DueAt: due, Now: now,
	})
	if d.Action != intent.CatchUpDoNow || !d.MergedWake || d.MissedTicks != 3 {
		t.Fatalf("unexpected: %+v", d)
	}
}

func TestOneShotStaleCancels(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	due := now.Add(-10 * 24 * time.Hour)
	d := intent.DecideCatchUp(intent.CatchUpInput{
		Kind: intent.IntentOneShot, DueAt: due, Now: now, StaleAfterDays: 7,
	})
	if d.Action != intent.CatchUpCancel || d.StaleDays < 7 {
		t.Fatalf("unexpected: %+v", d)
	}
}
