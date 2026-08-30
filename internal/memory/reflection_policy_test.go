package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

func TestRoleplayEvidenceCannotRewriteBond(t *testing.T) {
	root := t.TempDir()
	store, _ := NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	scope := identity.LocalCLI()
	ep, _ := episodes.WriteEpisode(context.Background(), scope, EpisodeWrite{
		Kind: EpisodePreference, Content: "角色扮演夹具：故事中的人物要求永远简短回答。",
		ExperienceMode: ExperienceSimulatedRoleplay, PersonIDs: []string{scope.PersonID()},
	})
	seed, _ := store.Create(context.Background(), scope, ReflectionSeedWrite{
		Scope: ReflectionScopeBond, Statement: "用户是否永远偏好简短回答", SourceType: ReflectionSourceInferred,
	})
	model := &scriptedReflectionModel{responses: []string{
		fmt.Sprintf(`{"worth_reflecting":true,"seed_ids":[%q],"queries":["简短回答"]}`, seed.ID),
		fmt.Sprintf(`{"read_ids":[%q]}`, ep.ID),
		fmt.Sprintf(`{"evidence":[{"seed_id":%q,"episode_id":%q,"state":"support"}],"decisions":[{"seed_id":%q,"action":"revise","kind":"bond","field":"basics","suggested_text":"永远简短回答"}]}`, seed.ID, ep.ID, seed.ID),
	}}
	run, err := (&ReflectionEngine{Chat: model, Store: store, Episodes: episodes, Proposals: proposals, Mode: ReflectionModeAgent}).Run(context.Background(), scope, "room", ReflectionTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Decisions) != 1 || run.Decisions[0].Action != ReflectionDefer || run.Decisions[0].ProposalID != "" {
		t.Fatalf("roleplay escaped isolation: %+v", run.Decisions)
	}
}

func TestRollbackRefusesToOverwriteLaterBondVersion(t *testing.T) {
	root := t.TempDir()
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	ledger, _ := NewMutationLedger(filepath.Join(root, "mutations"))
	scope := identity.LocalCLI()
	g := &reflectionTestGraph{bond: graph.Bond{SelfID: scope.AgentID, PersonID: scope.PersonID(), Version: 3, Basics: "新认识"}}
	original, err := ledger.Append(Mutation{
		Kind: "bond_patch", PersonID: scope.PersonID(), Field: "basics", BeforeVersion: 2, AfterVersion: 3,
		BeforeBond: SnapshotBond(graph.Bond{SelfID: scope.AgentID, PersonID: scope.PersonID(), Version: 2}),
		AfterBond:  SnapshotBond(g.bond), Actor: "dream",
	})
	if err != nil {
		t.Fatal(err)
	}
	g.bond.Version = 4
	dreamer := &Dreamer{Proposals: proposals, Graph: g, Ledger: ledger}
	if _, err := dreamer.RevertMutation(context.Background(), scope, original.ID, "try"); err == nil {
		t.Fatal("expected rollback version conflict")
	}
	if g.bond.Version != 4 || g.bond.Basics != "新认识" {
		t.Fatalf("later state overwritten: %+v", g.bond)
	}
}
