package memory

import (
	"context"
	"path/filepath"
	"testing"

	"deep-seeing/internal/identity"
)

func TestReflectionDirtyTracksSeedsEpisodesAndProposals(t *testing.T) {
	root := t.TempDir()
	reflections, _ := NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	scope := identity.LocalCLI()
	episodes.OnChanged = func(Episode) { _ = reflections.MarkDirty(scope.PersonID(), "episode") }
	proposals.OnChanged = func(BondProposal) { _ = reflections.MarkDirty(scope.PersonID(), "proposal") }

	if _, err := reflections.Create(context.Background(), scope, ReflectionSeedWrite{Scope: ReflectionScopeBond, Statement: "虚构问题", SourceType: ReflectionSourceInferred}); err != nil {
		t.Fatal(err)
	}
	state, err := reflections.IsDirty(scope.PersonID())
	if err != nil || !state.Dirty {
		t.Fatalf("seed dirty=%+v err=%v", state, err)
	}
	if err := reflections.ClearDirty(scope.PersonID()); err != nil {
		t.Fatal(err)
	}
	if _, err := episodes.WriteEpisode(context.Background(), scope, EpisodeWrite{Content: "虚构真实经历", PersonIDs: []string{scope.PersonID()}}); err != nil {
		t.Fatal(err)
	}
	state, _ = reflections.IsDirty(scope.PersonID())
	if !state.Dirty {
		t.Fatal("episode did not mark reflection dirty")
	}
	_ = reflections.ClearDirty(scope.PersonID())
	if _, err := proposals.Enqueue(context.Background(), scope, ProposalWrite{Kind: ProposalKindBond, Field: "basics", SuggestedText: "虚构提案"}); err != nil {
		t.Fatal(err)
	}
	state, _ = reflections.IsDirty(scope.PersonID())
	if !state.Dirty {
		t.Fatal("proposal did not mark reflection dirty")
	}
}
