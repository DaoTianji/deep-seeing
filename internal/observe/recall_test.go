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
