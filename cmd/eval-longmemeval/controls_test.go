package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	production "deep-seeing/internal/tools"
	"github.com/cloudwego/eino/components/tool"
)

func TestBM25RanksRelevantOlderSessionAheadOfRecentNoise(t *testing.T) {
	s := newBM25Search(nil, []memory.Episode{
		{ID: "old", Content: "I studied astronomy and purchased a telescope."},
		{ID: "new", Content: "I went grocery shopping and bought some milk."},
	})
	got := s.rank("astronomy telescope", 1)
	if len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("rank=%v", got)
	}
	if len(s.rank("unfindabletoken", 8)) != 0 {
		t.Fatal("no recent fallback permitted")
	}
	if s.rank("", 1)[0].ID != "new" {
		t.Fatal("empty query must retain newest-first behavior")
	}
}

func TestBM25StableAndCaseInsensitive(t *testing.T) {
	s := newBM25Search(nil, []memory.Episode{{ID: "a", Content: "École 1937 telescope"}, {ID: "b", Content: "École 1937 telescope"}})
	for i := 0; i < 100; i++ {
		got := s.rank("ÉCOLE TELESCOPE 1937", 1)
		if len(got) != 1 || got[0].ID != "b" {
			t.Fatalf("unstable tie %v", got)
		}
	}
	if len(newBM25Search(nil, nil).rank("x", 8)) != 0 {
		t.Fatal("empty corpus")
	}
}

func TestFullContextIsChronologicalLosslessAndLabelFree(t *testing.T) {
	long := strings.Repeat("historical-content ", 20000) + "END_MARKER"
	x := Item{ID: "secret-id", Type: "secret-type", Answer: json.RawMessage(`"SECRET_GOLD"`),
		Evidence: []string{"secret-source"}, SessionIDs: []string{"secret-source", "secret-source2"},
		Question: "What happened?", Date: "2023/05/30 (Tue) 23:40",
		Dates:    []string{"2023/05/20 (Sat) 02:21", "2023/05/19 (Fri) 02:21"},
		Sessions: [][]Turn{{{Role: "user", Content: long}}, {{Role: "assistant", Content: "EARLIER_MARKER"}}}}
	out := fullContext(x.input())
	if !strings.Contains(out, long) || strings.Index(out, "EARLIER_MARKER") > strings.Index(out, "END_MARKER") {
		t.Fatal("history truncated or unordered")
	}
	for _, secret := range []string{"secret-id", "secret-type", "SECRET_GOLD", "secret-source"} {
		if strings.Contains(out, secret) {
			t.Fatal("label leakage", secret)
		}
	}
	if !strings.HasSuffix(out, message(x.input())) {
		t.Fatal("question must follow complete history")
	}
}

func TestBM25ToolPreservesProductionCardsAndRecordsCandidates(t *testing.T) {
	ctx := context.Background()
	scope := identity.TenantScope{UserID: "benchmark", AgentID: "controls-test"}
	store, err := memory.NewEpisodeStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ep, err := store.WriteEpisode(ctx, scope, memory.EpisodeWrite{Kind: memory.EpisodeEvent, Content: "telescope " + strings.Repeat("visible history ", 30), PersonIDs: []string{scope.PersonID()}})
	if err != nil {
		t.Fatal(err)
	}
	all, err := production.All(production.Deps{Scope: scope, Episodes: store, RecallMode: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	var original tool.InvokableTool
	for _, candidate := range all {
		info, e := candidate.Info(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if info.Name == "search_episodes" {
			original = candidate.(tool.InvokableTool)
		}
	}
	if original == nil {
		t.Fatal("production tool missing")
	}
	input := `{"query":"telescope","limit":8}`
	want, err := original.InvokableRun(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	ep, err = store.Get(ctx, ep.ID)
	if err != nil {
		t.Fatal(err)
	}
	recorded, collector := observe.WithRecallCollector(ctx)
	got, err := newBM25Search(original, []memory.Episode{ep}).InvokableRun(recorded, input)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("candidate format differs\n%s\n%s", got, want)
	}
	if strings.Contains(got, ep.Content) {
		t.Fatal("search exposed complete body")
	}
	events := collector.Searches()
	if len(events) != 1 || len(events[0].ResultIDs) != 1 || events[0].ResultIDs[0] != ep.ID {
		t.Fatal("missing search evidence event")
	}
}
