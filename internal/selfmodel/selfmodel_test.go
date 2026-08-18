package selfmodel_test

import (
	"context"
	"path/filepath"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/selfmodel"
)

func TestEvidenceRoleplayCannotPromote(t *testing.T) {
	modes := []memory.ExperienceMode{memory.ExperienceSimulatedRoleplay, memory.ExperienceStoryReading}
	if selfmodel.CanPromoteToPrinciple(modes) {
		t.Fatal("roleplay-only must not promote")
	}
	if selfmodel.MaxStatusForModes(modes) != selfmodel.StatusTentative {
		t.Fatal("max status should be tentative")
	}
}

func TestEvidenceRealInteractionCanPromote(t *testing.T) {
	modes := []memory.ExperienceMode{memory.ExperienceRealInteraction, memory.ExperienceSimulatedRoleplay}
	if !selfmodel.CanPromoteToPrinciple(modes) {
		t.Fatal("expected promote allowed")
	}
}

func TestStoreCreateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := selfmodel.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.Create(selfmodel.Write{
		Type: selfmodel.TypePattern, Status: selfmodel.StatusClaimed,
		Title: "责任倾向", Body: "角色代入中多次选择承担责任。",
		ExperienceModes: []memory.ExperienceMode{memory.ExperienceSimulatedRoleplay},
		Actor:           "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != selfmodel.StatusTentative {
		t.Fatalf("claimed should be capped to tentative, got %s", a.Status)
	}
	got, err := store.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "责任倾向" || got.Body == "" || len(got.Revisions) == 0 {
		t.Fatalf("roundtrip failed: %+v", got)
	}
}

func TestInspectAndTrace(t *testing.T) {
	dir := t.TempDir()
	store, err := selfmodel.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.Create(selfmodel.Write{
		Type: selfmodel.TypeTension, Title: "自由 vs 责任", Body: "两股力同时存在",
		ExperienceModes: []memory.ExperienceMode{memory.ExperienceRealInteraction},
	})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := store.Inspect(10)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Counts["tension"] < 1 {
		t.Fatalf("counts=%v", sum.Counts)
	}
	tr, err := store.TraceBelief(a.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Artifact.ID != a.ID || !tr.CanBePrinciple {
		t.Fatalf("trace=%+v", tr)
	}
	tens, err := store.ListTensions(5)
	if err != nil || len(tens) != 1 {
		t.Fatalf("tensions=%v err=%v", tens, err)
	}
}

func TestApplyAcceptedProposal(t *testing.T) {
	dir := t.TempDir()
	store, err := selfmodel.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.ApplyAcceptedProposal(context.Background(), identity.LocalCLI(), memory.BondProposal{
		ID: "prop_x", Kind: memory.ProposalKindPrinciple, SuggestedText: "原则A",
		ExperienceModes: []memory.ExperienceMode{memory.ExperienceSimulatedRoleplay},
	}, nil)
	if err == nil {
		t.Fatal("expected principle reject")
	}
	res, err := store.ApplyAcceptedProposal(context.Background(), identity.LocalCLI(), memory.BondProposal{
		ID: "prop_y", Kind: memory.ProposalKindSelfPattern, Field: "压力模式",
		SuggestedText: "压力下收紧", ExperienceModes: []memory.ExperienceMode{memory.ExperienceSimulatedRoleplay},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Artifact.Status != selfmodel.StatusTentative {
		t.Fatalf("status=%s", res.Artifact.Status)
	}
}

func TestDreamAcceptSelfWritesLedger(t *testing.T) {
	dir := t.TempDir()
	store, err := selfmodel.NewStore(filepath.Join(dir, "self"))
	if err != nil {
		t.Fatal(err)
	}
	props, err := memory.NewProposalStore(filepath.Join(dir, "props"))
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := memory.NewMutationLedger(filepath.Join(dir, "mut"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	p, err := props.Enqueue(context.Background(), scope, memory.ProposalWrite{
		Kind: memory.ProposalKindTension, Field: "开放张力", SuggestedText: "想靠近又怕失去边界",
		ExperienceModes: []memory.ExperienceMode{memory.ExperienceRealInteraction},
	})
	if err != nil {
		t.Fatal(err)
	}
	d := &memory.Dreamer{
		Proposals: props, Ledger: ledger, Model: "test",
		Self: selfmodel.DreamBridge{Store: store},
	}
	res, err := d.ApplyAccept(context.Background(), scope, p.ID, "dream_self", "值得保留")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.MutationIDs) == 0 {
		t.Fatal("expected mutation")
	}
	muts, _ := ledger.ListRecent(5)
	if len(muts) == 0 || muts[0].Kind != "self_artifact" {
		t.Fatalf("muts=%v", muts)
	}
}
