package memory_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/transcript"
)

func TestProposalEnqueueAndList(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewProposalStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	p, err := store.Enqueue(context.Background(), scope, memory.ProposalWrite{
		Field: "style", SuggestedText: "更直接", Mode: "replace", Hypothesis: memory.HypothesisH1,
		Rationale: "本会话多次表现出直接风格", Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode != "append" {
		t.Fatalf("high field mode should force append, got %q", p.Mode)
	}
	if p.Kind != memory.ProposalKindBond {
		t.Fatalf("default kind=%s", p.Kind)
	}
	if !strings.HasPrefix(p.ID, "prop_") {
		t.Fatalf("id=%s", p.ID)
	}
	open, err := store.ListOpen(context.Background(), scope, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].ID != p.ID {
		t.Fatalf("open=%v", open)
	}
	recall := memory.FormatOpenRecall(open)
	if !strings.Contains(recall, "更直接") {
		t.Fatalf("recall=%s", recall)
	}
	if _, err := os.Stat(filepath.Join(dir, "open", p.ID+".md")); err != nil {
		t.Fatal(err)
	}
}

func TestProposalKindRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewProposalStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := store.Enqueue(context.Background(), identity.LocalCLI(), memory.ProposalWrite{
		Kind: memory.ProposalKindPrinciple, SuggestedText: "自由与责任绑定", Rationale: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != memory.ProposalKindPrinciple {
		t.Fatalf("kind=%s", p.Kind)
	}
	got, err := store.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != memory.ProposalKindPrinciple {
		t.Fatalf("reload kind=%s", got.Kind)
	}
}

func TestSessionReviewSkipsShort(t *testing.T) {
	dir := t.TempDir()
	eps, err := memory.NewEpisodeStore(filepath.Join(dir, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	props, err := memory.NewProposalStore(filepath.Join(dir, "proposals"))
	if err != nil {
		t.Fatal(err)
	}
	r := &memory.SessionReviewer{
		Chat: &memory.ChatClient{}, Episodes: eps, Proposals: props, MinChars: 80,
	}
	res, err := r.Run(context.Background(), identity.LocalCLI(), "cli", []transcript.Message{
		transcript.User("hi"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped {
		t.Fatal("expected skip")
	}
}

func TestNormalizeHighFieldProposalMode(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewProposalStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := store.Enqueue(context.Background(), identity.LocalCLI(), memory.ProposalWrite{
		Field: "boundaries", SuggestedText: "不喜欢被催", Mode: "replace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode != "append" {
		t.Fatalf("got %q", p.Mode)
	}
}
