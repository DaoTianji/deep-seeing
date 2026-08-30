package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"deep-seeing/internal/identity"
)

type fakeTensionResolver struct{ called bool }

func (r *fakeTensionResolver) ResolveTension(_ context.Context, _ identity.TenantScope, artifactID, summary string, sourceEpisodeIDs []string) (string, map[string]any, map[string]any, error) {
	r.called = true
	if artifactID == "" || summary == "" || len(sourceEpisodeIDs) == 0 {
		return "", nil, nil, fmt.Errorf("incomplete resolution")
	}
	return artifactID, map[string]any{"status": "active"}, map[string]any{"status": "deprecated"}, nil
}

func TestReflectionAgentResolvesTensionWithEvidenceLedger(t *testing.T) {
	root := t.TempDir()
	store, _ := NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	ledger, _ := NewMutationLedger(filepath.Join(root, "mutations"))
	scope := identity.LocalCLI()
	ep, _ := episodes.WriteEpisode(context.Background(), scope, EpisodeWrite{
		Kind: EpisodeEvent, Content: "虚构真实经历：用户解释了两个选择对应的不同情境。",
		ExperienceMode: ExperienceRealInteraction, PersonIDs: []string{scope.PersonID()},
	})
	seed, _ := store.Create(context.Background(), scope, ReflectionSeedWrite{
		Scope: ReflectionScopeTension, Statement: "两个选择为何不同", SourceType: ReflectionSourceInferred,
	})
	model := &scriptedReflectionModel{responses: []string{
		fmt.Sprintf(`{"worth_reflecting":true,"seed_ids":[%q],"queries":["选择 情境"]}`, seed.ID),
		fmt.Sprintf(`{"read_ids":[%q]}`, ep.ID),
		fmt.Sprintf(`{"evidence":[{"seed_id":%q,"episode_id":%q,"state":"support"}],"decisions":[{"seed_id":%q,"action":"resolve_tension","kind":"tension","field":"tension-1","suggested_text":"差异来自任务风险","reason_summary":"用户已明确情境"}]}`, seed.ID, ep.ID, seed.ID),
	}}
	resolver := &fakeTensionResolver{}
	run, err := (&ReflectionEngine{
		Chat: model, Store: store, Episodes: episodes, Proposals: proposals, Ledger: ledger,
		Tensions: resolver, Mode: ReflectionModeAgent,
	}).Run(context.Background(), scope, "room", ReflectionTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if !resolver.called || len(run.MutationIDs) != 1 || run.Decisions[0].Action != ReflectionResolveTension {
		t.Fatalf("run=%+v called=%v", run, resolver.called)
	}
	mutation, err := ledger.Get(run.MutationIDs[0])
	if err != nil || mutation.Kind != "tension_resolution" || mutation.ReflectionSeedID != seed.ID || len(mutation.SourceEpisodeIDs) != 1 {
		t.Fatalf("mutation=%+v err=%v", mutation, err)
	}
}
