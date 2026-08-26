package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

type fakeNormReader struct {
	bond  graph.Bond
	err   error
	calls int
}

func (f *fakeNormReader) GetBond(context.Context, identity.TenantScope, string) (graph.Bond, error) {
	f.calls++
	return f.bond, f.err
}

func TestNormSnapshotCachesCompleteActiveBond(t *testing.T) {
	scope := identity.LocalCLI()
	reader := &fakeNormReader{bond: graph.Bond{
		PersonID: scope.PersonID(),
		Version:  7,
		Items: []graph.BondItem{
			{ID: "b", Slot: graph.SlotBasics, Claim: "称呼偏好是安", Status: "active"},
			{ID: "i", Slot: graph.SlotInteraction, Claim: "先给结论", Status: "active"},
			{ID: "bd", Slot: graph.SlotBoundaries, Claim: "不要擅自记录隐私", Status: "active"},
			{ID: "p", Slot: graph.SlotPriorities, Claim: "重视自主性", Status: "active"},
			{ID: "bl", Slot: graph.SlotBaseline, Claim: "通常愿意慢慢讨论", Status: "active"},
			{ID: "old", Slot: graph.SlotBasics, Claim: "已经退休", Status: "retired"},
		},
		StrategyCache:    "先确认目标再展开",
		StrategyCacheVer: 7,
	}}
	cache := NewNormSnapshotCache(reader, scope)

	first, err := cache.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 {
		t.Fatalf("reader calls=%d, want 1", reader.calls)
	}
	if first.BondVersion != 7 || second.BondVersion != 7 || first.Placeholder {
		t.Fatalf("unexpected snapshots: first=%+v second=%+v", first, second)
	}
	for _, want := range []string{"称呼偏好是安", "先给结论", "不要擅自记录隐私", "重视自主性", "通常愿意慢慢讨论", "先确认目标再展开"} {
		if !strings.Contains(first.Text, want) {
			t.Fatalf("full norm missing %q: %s", want, first.Text)
		}
	}
	if strings.Contains(first.Text, "已经退休") {
		t.Fatalf("retired item leaked: %s", first.Text)
	}

	reader.bond.Version = 8
	cache.Invalidate()
	third, err := cache.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 2 || third.BondVersion != 8 {
		t.Fatalf("invalidate did not reload: calls=%d snapshot=%+v", reader.calls, third)
	}
	if strings.Contains(third.Text, "先确认目标再展开") {
		t.Fatalf("stale strategy leaked after version bump: %s", third.Text)
	}
}

func TestNormSnapshotFallsBackToPlaceholder(t *testing.T) {
	scope := identity.LocalCLI()
	reader := &fakeNormReader{err: errors.New("graph offline")}
	cache := NewNormSnapshotCache(reader, scope)

	snapshot, err := cache.Snapshot(context.Background())
	if err == nil {
		t.Fatal("expected graph error for observability")
	}
	if !snapshot.Placeholder || snapshot.Text != graph.BondPlaceholder {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if _, err := cache.Snapshot(context.Background()); err != nil {
		t.Fatalf("cached placeholder should keep chat usable: %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("reader calls=%d, want 1", reader.calls)
	}
}
