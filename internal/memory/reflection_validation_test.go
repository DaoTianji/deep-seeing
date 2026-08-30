package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/identity"
)

func TestReflectionEvidenceNormalizationProtectsCorrectionsAndTensions(t *testing.T) {
	episode := Episode{ID: "ep_1", ExperienceMode: ExperienceRealInteraction, Metadata: map[string]string{}}
	for _, tt := range []struct {
		name       string
		seed       ReflectionSeed
		correction bool
		want       ReflectionEvidenceState
	}{
		{name: "direct correction uses history as context", seed: ReflectionSeed{ID: "ref_1", Scope: ReflectionScopeBond, SourceType: ReflectionSourceDirect}, correction: true, want: ReflectionEvidenceContext},
		{name: "new explicit episode may support inferred correction", seed: ReflectionSeed{ID: "ref_1", Scope: ReflectionScopeBond, SourceType: ReflectionSourceInferred}, correction: true, want: ReflectionEvidenceSupport},
		{name: "unresolved tension has context not supporting conclusion", seed: ReflectionSeed{ID: "ref_1", Scope: ReflectionScopeTension, SourceType: ReflectionSourceInferred}, want: ReflectionEvidenceContext},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"evidence":[{"seed_id":"ref_1","episode_id":"ep_1","state":"support"}],"decisions":[{"seed_id":"ref_1","current_correction":%t}]}`, tt.correction)
			var out reflectionFinalOut
			if err := json.Unmarshal([]byte(raw), &out); err != nil {
				t.Fatal(err)
			}
			got := validateReflectionEvidence(out, []ReflectionSeed{tt.seed}, map[string]Episode{episode.ID: episode}, map[string]Episode{episode.ID: episode})
			if len(got) != 1 || got[0].State != tt.want {
				t.Fatalf("evidence=%+v want=%s", got, tt.want)
			}
		})
	}
}

func TestReflectionConfirmRequiresEvidenceAndWritesSupportLedger(t *testing.T) {
	root := t.TempDir()
	store, _ := NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	ledger, _ := NewMutationLedger(filepath.Join(root, "mutations"))
	scope := identity.LocalCLI()
	ep, _ := episodes.WriteEpisode(context.Background(), scope, EpisodeWrite{
		Kind: EpisodePreference, Content: "虚构证据：用户明确偏好结构化解释。", ExperienceMode: ExperienceRealInteraction,
		PersonIDs: []string{scope.PersonID()},
	})
	seed, _ := store.Create(context.Background(), scope, ReflectionSeedWrite{
		Scope: ReflectionScopeBond, Statement: "用户偏好结构化解释", SourceType: ReflectionSourceInferred,
	})
	model := &scriptedReflectionModel{responses: []string{
		fmt.Sprintf(`{"worth_reflecting":true,"seed_ids":[%q],"queries":["结构化解释"]}`, seed.ID),
		fmt.Sprintf(`{"read_ids":[%q]}`, ep.ID),
		fmt.Sprintf(`{"evidence":[{"seed_id":%q,"episode_id":%q,"state":"support"}],"decisions":[{"seed_id":%q,"action":"confirm","kind":"bond","reason_summary":"重复明确表达"}]}`, seed.ID, ep.ID, seed.ID),
	}}
	run, err := (&ReflectionEngine{Chat: model, Store: store, Episodes: episodes, Proposals: proposals, Ledger: ledger, Mode: ReflectionModeAgent, Model: "test"}).Run(context.Background(), scope, "room", ReflectionTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.MutationIDs) != 1 || len(run.Decisions) != 1 || run.Decisions[0].Action != ReflectionConfirm {
		t.Fatalf("run=%+v", run)
	}
	records, _ := ledger.ListRecent(10)
	if len(records) != 1 || records[0].Kind != "reflection_confirm" || records[0].ReflectionSeedID != seed.ID || len(records[0].SourceEpisodeIDs) != 1 || records[0].SourceEpisodeIDs[0] != ep.ID {
		t.Fatalf("records=%+v", records)
	}
}

func TestReflectionWithoutEligibleSeedKeepsDirty(t *testing.T) {
	root := t.TempDir()
	store, _ := NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	scope := identity.LocalCLI()
	if err := store.MarkDirty(scope.PersonID(), "episode"); err != nil {
		t.Fatal(err)
	}
	run, err := (&ReflectionEngine{Chat: &scriptedReflectionModel{}, Store: store, Episodes: episodes, Proposals: proposals, Mode: ReflectionModeObserve}).Run(context.Background(), scope, "room", ReflectionTriggerManual)
	if err != nil || !run.NoChange || !strings.Contains(run.Notes, "No open") {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	dirty, _ := store.IsDirty(scope.PersonID())
	if !dirty.Dirty {
		t.Fatal("dirty marker was consumed without an eligible seed")
	}
}

func TestGeneratedStatementsRemainTentative(t *testing.T) {
	if got := tentativeGeneratedStatement("安在边界上过度退让。"); !strings.Contains(got, "是否可能") || !strings.HasSuffix(got, "？") {
		t.Fatalf("not tentative: %q", got)
	}
	if got := tentativeGeneratedStatement("是否可能存在另一种解释？"); got != "是否可能存在另一种解释？" {
		t.Fatalf("already tentative statement changed: %q", got)
	}
}

func TestNoChangeIsDerivedFromConsolidatingActions(t *testing.T) {
	if hasConsolidatingDecision([]ReflectionDecision{{Action: ReflectionRejectSeed}, {Action: ReflectionDefer}}) {
		t.Fatal("reject and defer must not be reported as a long-term cognition change")
	}
	if !hasConsolidatingDecision([]ReflectionDecision{{Action: ReflectionRevise}}) {
		t.Fatal("revision must be reported as a consolidation change")
	}
}
