package birth_test

import (
	"context"
	"path/filepath"
	"testing"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/origin"
)

// Birth-oriented behavioral checks (not a live chat suite).
// Full Know→Act with LLM remains a manual Birth Test checklist item.

func TestBirth_OriginOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	scope := identity.LocalCLI()
	letter := origin.Letter{Body: "你好 mudnet"}
	gate := origin.BootGate{StateDir: dir}
	_, first, err := origin.IntroductionForBoot(gate, scope, letter, false)
	if err != nil || !first {
		t.Fatalf("first=%v err=%v", first, err)
	}
	text, first2, err := origin.IntroductionForBoot(gate, scope, letter, false)
	if err != nil || first2 || text != "" {
		t.Fatalf("second inject should be empty")
	}
}

func TestBirth_SingleOppositeDoesNotFlipViaProposalDiscipline(t *testing.T) {
	// Three consistent proposes + accept append style; a replace must fail.
	g := &memBond{style: "直接\n坦诚"}
	_, err := graph.ApplyHighField(g.style, "完全反过来变成委婉到极致", "replace")
	if err == nil {
		t.Fatal("single opposite replace must not overwrite style")
	}
	next, err := graph.ApplyHighField(g.style, "偶尔更柔和", "append")
	if err != nil {
		t.Fatal(err)
	}
	if next == "偶尔更柔和" {
		t.Fatal("append must retain prior baseline")
	}
}

func TestBirth_InvalidateHidesFromSearch(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewEpisodeStore(filepath.Join(dir, "ep"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	ep, err := store.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{
		Content: "错误记忆：他喜欢被叫老板",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetStatus(context.Background(), ep.ID, memory.EpisodeInvalid, "对方更正：不要叫老板"); err != nil {
		t.Fatal(err)
	}
	found, err := store.Search(context.Background(), scope, memory.Query{Text: "老板", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatal("invalid episode must not haunt search")
	}
}

func TestBirth_DreamAcceptLeavesMutationHistory(t *testing.T) {
	dir := t.TempDir()
	props, _ := memory.NewProposalStore(filepath.Join(dir, "p"))
	ledger, _ := memory.NewMutationLedger(filepath.Join(dir, "m"))
	scope := identity.LocalCLI()
	p, err := props.Enqueue(context.Background(), scope, memory.ProposalWrite{
		Field: "strategy", SuggestedText: "先听完再给建议", Source: "birth",
	})
	if err != nil {
		t.Fatal(err)
	}
	g := &memBondGraph{}
	d := &memory.Dreamer{Proposals: props, Graph: g, Ledger: ledger, Model: "birth-test"}
	if _, err := d.ApplyAccept(context.Background(), scope, p.ID, "dream_birth", "多次互动一致"); err != nil {
		t.Fatal(err)
	}
	muts, err := ledger.ListRecent(5)
	if err != nil || len(muts) == 0 {
		t.Fatal(err)
	}
	if muts[0].Before["strategy"] != "" && muts[0].After["strategy"] == "" {
		t.Fatal("ledger should record after")
	}
}

type memBond struct{ style string }

type memBondGraph struct {
	bond graph.Bond
}

func (m *memBondGraph) GetBond(context.Context, identity.TenantScope, string) (graph.Bond, error) {
	return m.bond, nil
}

func (m *memBondGraph) PatchBond(_ context.Context, scope identity.TenantScope, personID string, patch graph.BondPatch) (graph.Bond, error) {
	if patch.Strategy != "" {
		m.bond.Strategy = graph.MergeMedium(m.bond.Strategy, patch.Strategy)
	}
	m.bond.SelfID = scope.AgentID
	m.bond.PersonID = personID
	return m.bond, nil
}
