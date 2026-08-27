package attention_test

import (
	"testing"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
)

func TestSessionWorkspaceRequiresExplicitReplacement(t *testing.T) {
	store := attention.NewSessionStore(attention.Capacity{Center: 1, Support: 1, Periphery: 1})
	first, err := store.Apply("s1", []attention.Decision{{Source: contextsource.Workspace, ID: "w1", Target: attention.Center}})
	if err != nil || len(first.Items) != 1 {
		t.Fatalf("first apply: snapshot=%+v err=%v", first, err)
	}
	if _, err := store.Apply("s1", []attention.Decision{{Source: contextsource.Episode, ID: "e1", Target: attention.Center}}); err == nil {
		t.Fatal("full center accepted an implicit eviction")
	}
	unchanged := store.Snapshot("s1")
	if len(unchanged.Items) != 1 || unchanged.Items[0].ID != "w1" {
		t.Fatalf("failed apply mutated workspace: %+v", unchanged)
	}
	replaced, err := store.Apply("s1", []attention.Decision{{
		Source: contextsource.Episode, ID: "e1", Target: attention.Center,
		ReplaceSource: contextsource.Workspace, ReplaceID: "w1",
	}})
	if err != nil || len(replaced.Items) != 1 || replaced.Items[0].ID != "e1" {
		t.Fatalf("explicit replacement failed: snapshot=%+v err=%v", replaced, err)
	}
}

func TestSessionWorkspaceTiersIdleAndIsolation(t *testing.T) {
	store := attention.NewSessionStore(attention.DefaultCapacity())
	_, err := store.Apply("s1", []attention.Decision{
		{Source: contextsource.Workspace, ID: "w1", Target: attention.Center},
		{Source: contextsource.Intent, ID: "i1", Target: attention.Support},
		{Source: contextsource.Proposal, ID: "p1", Target: attention.Periphery},
	})
	if err != nil {
		t.Fatal(err)
	}
	advanced := store.BeginTurn("s1")
	if len(advanced.Items) != 3 {
		t.Fatalf("items=%+v", advanced.Items)
	}
	for _, item := range advanced.Items {
		if item.IdleTurns != 1 {
			t.Fatalf("idle turns not advanced: %+v", advanced.Items)
		}
	}
	if other := store.Snapshot("s2"); len(other.Items) != 0 {
		t.Fatalf("cross-session attention leak: %+v", other)
	}
	if _, err := store.Apply("s1", []attention.Decision{{Source: contextsource.Bond, ID: "person", Target: attention.Center}}); err == nil {
		t.Fatal("Bond incorrectly entered candidate competition")
	}
}

func TestSessionWorkspaceBatchIsAtomic(t *testing.T) {
	store := attention.NewSessionStore(attention.DefaultCapacity())
	if _, err := store.Apply("s", []attention.Decision{
		{Source: contextsource.Workspace, ID: "w1", Target: attention.Center},
		{Source: contextsource.Workspace, ID: "w1", Target: attention.Support},
	}); err == nil {
		t.Fatal("duplicate batch item accepted")
	}
	if snapshot := store.Snapshot("s"); len(snapshot.Items) != 0 {
		t.Fatalf("failed batch partially applied: %+v", snapshot)
	}
}
