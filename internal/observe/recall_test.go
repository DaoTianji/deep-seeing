package observe

import (
	"context"
	"strings"
	"testing"
)

func TestRecallCollectorIsTurnScopedAndTruncates(t *testing.T) {
	query := strings.Repeat("记", 130)
	var streamed []RecallSearchTrace
	ctx, collector := WithRecallCollector(context.Background(), func(event RecallSearchTrace) {
		streamed = append(streamed, event)
	})
	RecordRecallSearch(ctx, RecallSearchTrace{
		Query: query, Limit: 3, ResultCount: 1, ResultIDs: []string{"ep1"}, Error: strings.Repeat("错", 170),
	})
	RecordRecallSearch(context.Background(), RecallSearchTrace{Query: "outside"})

	got := collector.Searches()
	if len(got) != 1 {
		t.Fatalf("searches=%+v", got)
	}
	if len(streamed) != 1 || streamed[0].Query != got[0].Query {
		t.Fatalf("streamed=%+v got=%+v", streamed, got)
	}
	if len([]rune(got[0].Query)) != 121 || !strings.HasSuffix(got[0].Query, "…") {
		t.Fatalf("query not truncated: %q", got[0].Query)
	}
	if len([]rune(got[0].Error)) != 161 || !strings.HasSuffix(got[0].Error, "…") {
		t.Fatalf("error not truncated: %q", got[0].Error)
	}
	got[0].ResultIDs[0] = "mutated"
	if collector.Searches()[0].ResultIDs[0] != "ep1" {
		t.Fatal("collector returned mutable result IDs")
	}
}

func TestRecallCollectorTracksReadAndEvidenceLifecycle(t *testing.T) {
	var reads []RecallReadTrace
	var evidence []RecallEvidenceTrace
	ctx, collector := WithRecallHooks(context.Background(), RecallHooks{
		OnRead: func(event RecallReadTrace) { reads = append(reads, event) },
		OnEvidence: func(event RecallEvidenceTrace) {
			evidence = append(evidence, event)
		},
	})
	RecordRecallSearch(ctx, RecallSearchTrace{ResultCount: 2, ResultIDs: []string{"ep1", "ep2"}})
	RecordRecallRead(ctx, RecallReadTrace{EpisodeID: "ep1"})
	if err := RecordRecallEvidence(ctx, []RecallEvidenceTrace{
		{EpisodeID: "ep1", Status: "used"},
		{EpisodeID: "ep2", Status: "dismissed", Reason: "irrelevant"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(collector.Reads()) != 1 || len(reads) != 1 || reads[0].EpisodeID != "ep1" {
		t.Fatalf("reads collector=%+v callback=%+v", collector.Reads(), reads)
	}
	if len(collector.Evidence()) != 2 || len(evidence) != 2 {
		t.Fatalf("evidence collector=%+v callback=%+v", collector.Evidence(), evidence)
	}
}

func TestRecallEvidenceValidation(t *testing.T) {
	newTurn := func(read bool) context.Context {
		ctx, _ := WithRecallCollector(context.Background())
		RecordRecallSearch(ctx, RecallSearchTrace{ResultCount: 1, ResultIDs: []string{"ep1"}})
		if read {
			RecordRecallRead(ctx, RecallReadTrace{EpisodeID: "ep1"})
		}
		return ctx
	}
	cases := []struct {
		name   string
		ctx    context.Context
		events []RecallEvidenceTrace
	}{
		{name: "unsearched", ctx: newTurn(true), events: []RecallEvidenceTrace{{EpisodeID: "other", Status: "used"}}},
		{name: "unread used", ctx: newTurn(false), events: []RecallEvidenceTrace{{EpisodeID: "ep1", Status: "used"}}},
		{name: "invalid reason", ctx: newTurn(false), events: []RecallEvidenceTrace{{EpisodeID: "ep1", Status: "dismissed", Reason: "because"}}},
		{name: "conflicting duplicate", ctx: newTurn(true), events: []RecallEvidenceTrace{{EpisodeID: "ep1", Status: "used"}, {EpisodeID: "ep1", Status: "dismissed", Reason: "stale"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := RecordRecallEvidence(tc.ctx, tc.events); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
