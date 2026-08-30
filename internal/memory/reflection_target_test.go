package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"deep-seeing/internal/identity"
)

func TestReflectionRejectsTargetsOutsideT3WriteScope(t *testing.T) {
	for _, target := range []string{"world", "soul", "unknown"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			store, _ := NewReflectionStore(filepath.Join(root, "reflections"))
			episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
			proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
			scope := identity.LocalCLI()
			seed, _ := store.Create(context.Background(), scope, ReflectionSeedWrite{
				Scope: ReflectionScopeBond, Statement: "当前直接表达", SourceType: ReflectionSourceDirect,
			})
			model := &scriptedReflectionModel{responses: []string{
				fmt.Sprintf(`{"worth_reflecting":true,"seed_ids":[%q],"queries":["不存在的夹具"]}`, seed.ID),
				fmt.Sprintf(`{"decisions":[{"seed_id":%q,"action":"revise","kind":%q,"field":"basics","suggested_text":"越界写入"}]}`, seed.ID, target),
			}}
			run, err := (&ReflectionEngine{Chat: model, Store: store, Episodes: episodes, Proposals: proposals, Mode: ReflectionModeAgent}).Run(context.Background(), scope, "room", ReflectionTriggerManual)
			if err != nil {
				t.Fatal(err)
			}
			if len(run.Decisions) != 1 || run.Decisions[0].Action != ReflectionDefer || run.Decisions[0].ProposalID != "" {
				t.Fatalf("target escaped T3 scope: %+v", run.Decisions)
			}
			open, _ := proposals.ListOpen(context.Background(), scope, "", 10)
			if len(open) != 0 {
				t.Fatalf("unexpected proposal: %+v", open)
			}
		})
	}
}
