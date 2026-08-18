package agency_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"deep-seeing/internal/agency"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/runtime"
)

func TestStoreCreateAndDue(t *testing.T) {
	store, err := intent.OpenStore(filepath.Join(t.TempDir(), "rt"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	it, err := store.Create(context.Background(), intent.CreateInput{
		AgentID: "a1", Kind: intent.IntentOneShot, Title: "复看张力",
		DueAt: now.Add(-time.Hour), Body: "看看还在不在",
	})
	if err != nil {
		t.Fatal(err)
	}
	due, err := store.ListDue(context.Background(), "a1", now, 10)
	if err != nil || len(due) != 1 || due[0].ID != it.ID {
		t.Fatalf("due=%v err=%v", due, err)
	}
}

func TestRunnerCatchUpCancelStale(t *testing.T) {
	store, err := intent.OpenStore(filepath.Join(t.TempDir(), "rt"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	_, err = store.Create(context.Background(), intent.CreateInput{
		AgentID: "a1", Kind: intent.IntentOneShot, Title: "过期事项",
		DueAt: now.Add(-10 * 24 * time.Hour), StaleAfterDays: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := &agency.Runner{
		Store: store, Queue: runtime.NewExecutionQueue("a1"), Budget: agency.DefaultBudget(),
		Now: func() time.Time { return now },
	}
	res, err := r.ProcessDue(context.Background(), "a1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Result != "cancelled" {
		t.Fatalf("res=%+v", res)
	}
}

func TestRunnerRecurringNoopAndReschedule(t *testing.T) {
	store, err := intent.OpenStore(filepath.Join(t.TempDir(), "rt"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	it, err := store.Create(context.Background(), intent.CreateInput{
		AgentID: "a1", Kind: intent.IntentRecurring, Title: "周回顾",
		DueAt: now.Add(-time.Hour), Interval: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := &agency.Runner{
		Store: store, Queue: runtime.NewExecutionQueue("a1"), Budget: agency.DefaultBudget(),
		Now: func() time.Time { return now },
	}
	res, err := r.ProcessDue(context.Background(), "a1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Result != "noop" {
		t.Fatalf("res=%+v", res)
	}
	if !runtime.IsAutoSession(res[0].SessionID) {
		t.Fatalf("session=%s", res[0].SessionID)
	}
	got, err := store.Get(context.Background(), it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.DueAt.After(now) || got.Attempt != 1 {
		t.Fatalf("got=%+v", got)
	}
}

func TestBudgetExhaustion(t *testing.T) {
	b := &agency.Budget{MaxPerDay: 1, AllowOutbound: false}
	now := time.Now().UTC()
	ok, _ := b.AllowWake(now)
	if !ok {
		t.Fatal("first should allow")
	}
	b.ConsumeWake(now)
	ok, why := b.AllowWake(now)
	if ok {
		t.Fatalf("expected exhaust, why=%s", why)
	}
	snapshot := b.Snapshot(now)
	if snapshot.Used != 1 || snapshot.Remaining != 0 || snapshot.MaxPerDay != 1 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestSchedulerSnapshot(t *testing.T) {
	scheduler := &agency.Scheduler{Interval: 30 * time.Second, Limit: 3}
	snapshot := scheduler.Snapshot()
	if snapshot.Running || snapshot.IntervalSeconds != 30 || snapshot.Limit != 3 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}
