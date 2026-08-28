package observe_test

import (
	"testing"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/observe"
)

func TestSummarizeTurnHealthClassifiesRecoverableDegradation(t *testing.T) {
	snapshot := attention.Snapshot{Version: attention.Version}
	health := observe.SummarizeTurnHealth(observe.TurnTrace{
		RecallMode: "agent", Attention: &snapshot, AnswerPreview: "仍然给出了回答",
		ContextSources: []observe.ContextSourceTrace{
			{Source: contextsource.Bond, State: "unavailable"},
			{Source: contextsource.Intent, State: "error"},
		},
		RecallSearches: []observe.RecallSearchTrace{{Error: "search backend unavailable"}},
		RecallReads:    []observe.RecallReadTrace{{EpisodeID: "ep_1", Error: "read failed"}},
		Errors:         []string{"agent exceeds max steps"},
	})
	if health.Status != observe.TurnDegraded {
		t.Fatalf("status=%q issues=%+v", health.Status, health.Issues)
	}
	want := map[string]bool{
		"source_unavailable": true, "source_error": true, "search_error": true,
		"read_error": true, "step_limit": true,
	}
	for _, issue := range health.Issues {
		delete(want, issue.Code)
		if !issue.Recoverable {
			t.Fatalf("unexpected unrecoverable issue: %+v", issue)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing issue codes: %v; got=%+v", want, health.Issues)
	}
}

func TestSummarizeTurnHealthDistinguishesHealthyAndFailed(t *testing.T) {
	snapshot := attention.Snapshot{Version: attention.Version}
	healthy := observe.SummarizeTurnHealth(observe.TurnTrace{RecallMode: "agent", Attention: &snapshot, AnswerPreview: "ok"})
	if healthy.Status != observe.TurnHealthy || len(healthy.Issues) != 0 {
		t.Fatalf("healthy=%+v", healthy)
	}
	failed := observe.SummarizeTurnHealth(observe.TurnTrace{RecallMode: "agent", Errors: []string{"context deadline exceeded"}})
	if failed.Status != observe.TurnFailed {
		t.Fatalf("failed=%+v", failed)
	}
	codes := map[string]bool{}
	for _, issue := range failed.Issues {
		codes[issue.Code] = true
	}
	if !codes["attention_unavailable"] || !codes["timeout"] {
		t.Fatalf("failed issue codes=%v", codes)
	}
}
