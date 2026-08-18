package runtime_test

import (
	"testing"
	"time"

	"deep-seeing/internal/runtime"
)

func TestDirtyCooldownAndBudget(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	pol := runtime.DirtyPolicy{Cooldown: time.Hour, BudgetPerWindow: 2}
	sig := runtime.MarkDirty("a1", "new episodes", 3, 1, now, pol)
	ok, reason := runtime.ShouldOfferMaintenance(sig, now, pol)
	if !ok {
		t.Fatalf("expected offer: %s", reason)
	}
	sig = runtime.ConsumeOffer(sig, now, pol)
	ok, reason = runtime.ShouldOfferMaintenance(sig, now, pol)
	if ok {
		t.Fatalf("expected cooldown block, got %s", reason)
	}
	later := now.Add(2 * time.Hour)
	ok, reason = runtime.ShouldOfferMaintenance(sig, later, pol)
	if !ok {
		t.Fatalf("expected offer after cooldown: %s", reason)
	}
	sig = runtime.ConsumeOffer(sig, later, pol)
	sig = runtime.ConsumeOffer(sig, later.Add(2*time.Hour), pol)
	ok, reason = runtime.ShouldOfferMaintenance(sig, later.Add(4*time.Hour), pol)
	if ok {
		t.Fatalf("expected budget exhausted, got %s", reason)
	}
}
