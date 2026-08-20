package memory_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

type memGraph struct {
	bond graph.Bond
}

func (m *memGraph) GetBond(_ context.Context, _ identity.TenantScope, personID string) (graph.Bond, error) {
	b := m.bond
	b.PersonID = personID
	return b, nil
}

func (m *memGraph) PatchBond(_ context.Context, scope identity.TenantScope, personID string, patch graph.BondPatch) (graph.Bond, error) {
	cur := m.bond
	steps := []struct {
		slot string
		text string
	}{
		{graph.SlotBasics, patch.Basics},
		{graph.SlotInteraction, patch.Style},
		{graph.SlotBoundaries, patch.Boundaries},
		{graph.SlotPriorities, patch.Concerns},
		{graph.SlotBaseline, patch.Baseline},
	}
	for _, st := range steps {
		text := strings.TrimSpace(st.text)
		if text == "" {
			continue
		}
		next, _, err := graph.PrepareAppendItem(cur, graph.AppendItemSpec{Slot: st.slot, Claim: text, Source: "patch"})
		if err != nil {
			if strings.Contains(err.Error(), "item limit") {
				continue
			}
			return graph.Bond{}, err
		}
		cur = next
	}
	if patch.Strategy != "" {
		cur.Strategy = graph.MergeMedium(cur.Strategy, patch.Strategy)
		cur.Version++
	}
	cur.SelfID = scope.AgentID
	cur.PersonID = personID
	m.bond = cur
	return m.bond, nil
}

func TestDreamApplyAcceptWritesLedger(t *testing.T) {
	dir := t.TempDir()
	props, err := memory.NewProposalStore(filepath.Join(dir, "proposals"))
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := memory.NewMutationLedger(filepath.Join(dir, "mutations"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	p, err := props.Enqueue(context.Background(), scope, memory.ProposalWrite{
		Field: "basics", SuggestedText: "早期朋友", Hypothesis: memory.HypothesisH1, Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	g := &memGraph{}
	d := &memory.Dreamer{Proposals: props, Graph: g, Ledger: ledger, Model: "test-model"}
	res, err := d.ApplyAccept(context.Background(), scope, p.ID, "dream_test", "3 次独立互动")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Accepted) != 1 || g.bond.Basics != "早期朋友" {
		t.Fatalf("res=%+v bond=%+v", res, g.bond)
	}
	muts, err := ledger.ListRecent(10)
	if err != nil || len(muts) == 0 {
		t.Fatalf("muts=%v err=%v", muts, err)
	}
	if muts[0].ProposalID != p.ID || muts[0].Actor != "dream" {
		t.Fatalf("mut=%+v", muts[0])
	}
	open, _ := props.ListOpen(context.Background(), scope, "", 10)
	if len(open) != 0 {
		t.Fatal("proposal should be resolved")
	}
}

func TestDreamSkipsWhenNoProposals(t *testing.T) {
	dir := t.TempDir()
	props, err := memory.NewProposalStore(filepath.Join(dir, "proposals"))
	if err != nil {
		t.Fatal(err)
	}
	d := &memory.Dreamer{Proposals: props}
	res, err := d.Run(context.Background(), identity.LocalCLI(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped {
		t.Fatal("expected skip")
	}
}

func TestPatchBondAppendsInteractionItem(t *testing.T) {
	g := &memGraph{bond: graph.Bond{
		Style: "直接",
		Items: []graph.BondItem{
			{ID: "interaction:legacy:0", Slot: graph.SlotInteraction, Claim: "直接", Status: "active"},
		},
	}}
	bond, err := g.PatchBond(context.Background(), identity.LocalCLI(), "user:mudnet", graph.BondPatch{
		Style: "整段覆盖", StyleMode: "replace",
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range bond.Items {
		if it.Claim == "整段覆盖" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected append item, got %+v", bond.Items)
	}
}
