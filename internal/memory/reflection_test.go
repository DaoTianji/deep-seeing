package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

type scriptedReflectionModel struct {
	responses []string
	calls     int
}

func (m *scriptedReflectionModel) Complete(_ context.Context, _, _ string) (string, error) {
	if m.calls >= len(m.responses) {
		return "", fmt.Errorf("unexpected model call %d", m.calls+1)
	}
	response := m.responses[m.calls]
	m.calls++
	return response, nil
}

type reflectionTestGraph struct{ bond graph.Bond }

func (g *reflectionTestGraph) GetBond(_ context.Context, _ identity.TenantScope, personID string) (graph.Bond, error) {
	b := g.bond
	b.PersonID = personID
	return b, nil
}

func (g *reflectionTestGraph) PatchBond(_ context.Context, scope identity.TenantScope, personID string, patch graph.BondPatch) (graph.Bond, error) {
	cur := g.bond
	for _, item := range []struct{ slot, text string }{
		{graph.SlotBasics, patch.Basics}, {graph.SlotInteraction, patch.Style},
		{graph.SlotBoundaries, patch.Boundaries}, {graph.SlotPriorities, patch.Concerns},
		{graph.SlotBaseline, patch.Baseline},
	} {
		if strings.TrimSpace(item.text) == "" {
			continue
		}
		next, _, err := graph.PrepareAppendItem(cur, graph.AppendItemSpec{
			Slot: item.slot, Claim: item.text, Source: "reflection", SourceEpisodeID: patch.SourceEpisodeID,
		})
		if err != nil {
			return graph.Bond{}, err
		}
		cur = next
	}
	cur.SelfID, cur.PersonID = scope.AgentID, personID
	g.bond = cur
	return cur, nil
}

func (g *reflectionTestGraph) PersistBondState(_ context.Context, _ identity.TenantScope, _ string, b graph.Bond) (graph.Bond, error) {
	g.bond = b
	return b, nil
}

func TestReflectionModeParsing(t *testing.T) {
	for raw, want := range map[string]ReflectionMode{"": ReflectionModeAgent, "observe": ReflectionModeObserve, "agent": ReflectionModeAgent, "legacy": ReflectionModeLegacy} {
		got, valid := ParseReflectionMode(raw)
		if !valid || got != want {
			t.Fatalf("raw=%q got=%s valid=%v", raw, got, valid)
		}
	}
	if got, valid := ParseReflectionMode("surprise"); valid || got != ReflectionModeLegacy {
		t.Fatalf("invalid got=%s valid=%v", got, valid)
	}
}

func TestReflectionStoreLifecycleCheckpointAndTrace(t *testing.T) {
	store, err := NewReflectionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	seed, err := store.Create(context.Background(), scope, ReflectionSeedWrite{
		SessionID: "room", SourceTurnIDs: []string{"turn-1", "turn-1"}, Scope: ReflectionScopeBond,
		Statement: "也许一次疲惫被误认为偏好", SourceType: ReflectionSourceInferred,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seed.SourceTurnIDs) != 1 || seed.Status != ReflectionSeedOpen {
		t.Fatalf("seed=%+v", seed)
	}
	if err := store.SaveCheckpoint(ReviewCheckpoint{PersonID: scope.PersonID(), SessionID: "room", LastTurnID: "turn-1", MessageCount: 2}); err != nil {
		t.Fatal(err)
	}
	cp, err := store.LoadCheckpoint(scope.PersonID(), "room")
	if err != nil || cp.LastTurnID != "turn-1" {
		t.Fatalf("cp=%+v err=%v", cp, err)
	}
	if _, err := store.UpdateStatus(context.Background(), seed.ID, ReflectionSeedDeferred, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	run, err := store.AppendRun(ReflectionRun{PersonID: scope.PersonID(), Mode: ReflectionModeObserve, SeedIDs: []string{seed.ID}, NoChange: true})
	if err != nil || run.ID == "" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	runs, err := store.ListRuns(10)
	if err != nil || len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("runs=%+v err=%v", runs, err)
	}
}

func TestReflectionObserveRequiresReadEvidenceAndDoesNotLeakBody(t *testing.T) {
	root := t.TempDir()
	store, _ := NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	scope := identity.LocalCLI()
	ep, err := episodes.WriteEpisode(context.Background(), scope, EpisodeWrite{
		Kind: EpisodePreference, Content: "虚构夹具：用户明确说详细回答更有帮助。", ExperienceMode: ExperienceRealInteraction,
		PersonIDs: []string{scope.PersonID()}, SessionID: "old-session",
	})
	if err != nil {
		t.Fatal(err)
	}
	seed, _ := store.Create(context.Background(), scope, ReflectionSeedWrite{
		Scope: ReflectionScopeBond, Statement: "用户是否更偏好详细回答", SourceType: ReflectionSourceInferred,
	})
	model := &scriptedReflectionModel{responses: []string{
		fmt.Sprintf(`{"worth_reflecting":true,"seed_ids":[%q],"queries":["详细回答"],"notes":"search"}`, seed.ID),
		fmt.Sprintf(`{"read_ids":[%q],"notes":"read"}`, ep.ID),
		fmt.Sprintf(`{"no_change":false,"notes":"supported","evidence":[{"seed_id":%q,"episode_id":%q,"state":"support"}],"decisions":[{"seed_id":%q,"action":"revise","kind":"bond","field":"basics","suggested_text":"偏好有必要的详细说明","mode":"append","reason_summary":"历史明确表达支持"}]}`, seed.ID, ep.ID, seed.ID),
	}}
	engine := &ReflectionEngine{Chat: model, Store: store, Episodes: episodes, Proposals: proposals, Mode: ReflectionModeObserve}
	run, err := engine.Run(context.Background(), scope, "room", ReflectionTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.ReadIDs) != 1 || len(run.Evidence) != 1 || !run.Evidence[0].Read {
		t.Fatalf("run=%+v", run)
	}
	if len(run.Decisions) != 1 || run.Decisions[0].ProposalID == "" || run.Decisions[0].MutationID != "" {
		t.Fatalf("decisions=%+v", run.Decisions)
	}
	raw, _ := json.Marshal(run)
	if strings.Contains(string(raw), "虚构夹具") || strings.Contains(string(raw), ep.Content) {
		t.Fatalf("trace leaked episode body: %s", raw)
	}
	open, _ := proposals.ListOpen(context.Background(), scope, "", 10)
	if len(open) != 1 || len(open[0].SourceEpisodeIDs) != 1 || open[0].ReflectionSeedID != seed.ID {
		t.Fatalf("proposal=%+v", open)
	}
}

func TestReflectionRejectsDerivedInferenceAsSoleEvidence(t *testing.T) {
	root := t.TempDir()
	store, _ := NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := NewProposalStore(filepath.Join(root, "proposals"))
	scope := identity.LocalCLI()
	ep, _ := episodes.WriteEpisode(context.Background(), scope, EpisodeWrite{
		Kind: EpisodeStateObservation, Content: "Review 推断：用户也许偏好简短。", Why: "session_review",
		ExperienceMode: ExperienceSelfReflection, PersonIDs: []string{scope.PersonID()},
		Metadata: map[string]string{"epistemic": "derived_inference"},
	})
	seed, _ := store.Create(context.Background(), scope, ReflectionSeedWrite{
		Scope: ReflectionScopeBond, Statement: "用户偏好简短", SourceType: ReflectionSourceInferred,
	})
	model := &scriptedReflectionModel{responses: []string{
		fmt.Sprintf(`{"worth_reflecting":true,"seed_ids":[%q],"queries":["偏好简短"]}`, seed.ID),
		fmt.Sprintf(`{"read_ids":[%q]}`, ep.ID),
		fmt.Sprintf(`{"evidence":[{"seed_id":%q,"episode_id":%q,"state":"support"}],"decisions":[{"seed_id":%q,"action":"revise","kind":"bond","field":"basics","suggested_text":"偏好简短"}]}`, seed.ID, ep.ID, seed.ID),
	}}
	run, err := (&ReflectionEngine{Chat: model, Store: store, Episodes: episodes, Proposals: proposals, Mode: ReflectionModeAgent}).Run(context.Background(), scope, "room", ReflectionTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Decisions) != 1 || run.Decisions[0].Action != ReflectionDefer || run.Decisions[0].ProposalID != "" {
		t.Fatalf("decisions=%+v evidence=%+v", run.Decisions, run.Evidence)
	}
}

func TestReflectionAgentWritesEvidenceAndRollbackRestoresBond(t *testing.T) {
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
	g := &reflectionTestGraph{bond: graph.Bond{SelfID: scope.AgentID, PersonID: scope.PersonID(), Version: 2}}
	dreamer := &Dreamer{Proposals: proposals, Graph: g, Ledger: ledger, Model: "test"}
	model := &scriptedReflectionModel{responses: []string{
		fmt.Sprintf(`{"worth_reflecting":true,"seed_ids":[%q],"queries":["结构化解释"]}`, seed.ID),
		fmt.Sprintf(`{"read_ids":[%q]}`, ep.ID),
		fmt.Sprintf(`{"evidence":[{"seed_id":%q,"episode_id":%q,"state":"support"}],"decisions":[{"seed_id":%q,"action":"revise","kind":"bond","field":"basics","suggested_text":"偏好结构化解释","reason_summary":"明确历史证据"}]}`, seed.ID, ep.ID, seed.ID),
	}}
	engine := &ReflectionEngine{Chat: model, Store: store, Episodes: episodes, Proposals: proposals, Dreamer: dreamer, Graph: g, Mode: ReflectionModeAgent}
	run, err := engine.Run(context.Background(), scope, "room", ReflectionTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.MutationIDs) != 1 || g.bond.Basics != "偏好结构化解释" || g.bond.Version != 3 {
		t.Fatalf("run=%+v bond=%+v", run, g.bond)
	}
	mutation, err := ledger.Get(run.MutationIDs[0])
	if err != nil || mutation.ReflectionSeedID != seed.ID || len(mutation.SourceEpisodeIDs) != 1 || mutation.BeforeBond == nil {
		t.Fatalf("mutation=%+v err=%v", mutation, err)
	}
	reverted, err := dreamer.RevertMutation(context.Background(), scope, mutation.ID, "发现反思判断错误")
	if err != nil {
		t.Fatal(err)
	}
	if reverted.RevertsMutationID != mutation.ID || g.bond.Basics != "" || g.bond.Version != 4 {
		t.Fatalf("reverted=%+v bond=%+v", reverted, g.bond)
	}
}

func TestGenerativeDreamOnlyCreatesGeneratedSeeds(t *testing.T) {
	store, _ := NewReflectionStore(t.TempDir())
	scope := identity.LocalCLI()
	tension, _ := store.Create(context.Background(), scope, ReflectionSeedWrite{
		Scope: ReflectionScopeTension, Statement: "想靠近与怕打扰同时存在", SourceType: ReflectionSourceInferred,
	})
	model := &scriptedReflectionModel{responses: []string{
		`{"worth_dreaming":true,"dream_fragment":"两条路在夜里交汇。","hypotheses":[{"scope":"tension","statement":"是否把询问本身误认为打扰"}],"notes":"imagined"}`,
	}}
	run, err := (&GenerativeDreamer{Chat: model, Store: store, Mode: ReflectionModeAgent}).Run(context.Background(), scope, "room", ReflectionTriggerTension)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.GeneratedSeedIDs) != 1 || len(run.MutationIDs) != 0 || tension.ID == run.GeneratedSeedIDs[0] {
		t.Fatalf("run=%+v", run)
	}
	generated, err := store.Get(context.Background(), run.GeneratedSeedIDs[0])
	if err != nil || !generated.Generated || generated.SourceType != ReflectionSourceGenerated {
		t.Fatalf("generated=%+v err=%v", generated, err)
	}
}
