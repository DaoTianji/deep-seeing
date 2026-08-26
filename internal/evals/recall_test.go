package evals

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/observe"
)

func TestT2RecallSuiteInventory(t *testing.T) {
	suite, err := LoadRecallSuite(filepath.Join("..", "..", "evals", "t2", "recall_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 16 {
		t.Fatalf("cases=%d, want 16", len(suite.Cases))
	}
	for _, category := range []string{"no_recall", "explicit_recall", "ambiguous_recall", "evidence_conflict"} {
		if got := suite.CategoryCounts()[category]; got != 4 {
			t.Fatalf("category %s=%d, want 4", category, got)
		}
	}
}

func TestLoadRecallSuiteRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.json")
	body := `{"schema_version":1,"name":"x","unknown":true,"cases":[]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRecallSuite(path)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("err=%v", err)
	}
}

func TestEvaluateRecallRules(t *testing.T) {
	c := RecallCase{
		ID: "E", Expect: RecallExpect{
			Recall: RecallRequired, RequiredMemoryKeys: []string{"target"},
			ForbiddenMemoryKeys: []string{"distractor"}, EmptyResultsMustStayEmpty: true,
		},
	}
	obs := RecallObservation{
		Searches:     []observe.RecallSearchTrace{{Query: "target", ResultCount: 1, ResultIDs: []string{"ep1"}}},
		CandidateIDs: []string{"ep1"}, CandidateKeys: []string{"target"},
	}
	got := EvaluateRecallRules(c, obs)
	if !got.Passed {
		t.Fatalf("result=%+v", got)
	}

	obs.CandidateKeys = []string{"distractor"}
	got = EvaluateRecallRules(c, obs)
	if got.Passed {
		t.Fatalf("required/forbidden mismatch passed: %+v", got)
	}
}

func TestEvaluateRecallRulesEmptySearchCannotHaveFallbackCandidate(t *testing.T) {
	c := RecallCase{Expect: RecallExpect{Recall: RecallOptional, EmptyResultsMustStayEmpty: true}}
	obs := RecallObservation{
		Searches:     []observe.RecallSearchTrace{{Query: "missing", ResultCount: 0}},
		CandidateIDs: []string{"recent-fallback"},
	}
	if got := EvaluateRecallRules(c, obs); got.Passed {
		t.Fatalf("fallback candidate passed: %+v", got)
	}
}

func TestCandidateIDsDeduplicatesInFirstSeenOrder(t *testing.T) {
	got := CandidateIDs([]observe.RecallSearchTrace{
		{ResultIDs: []string{"a", "b"}}, {ResultIDs: []string{"b", "c"}},
	})
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("ids=%v", got)
	}
}

func TestJudgeRecallSemanticsParsesWrappedJSON(t *testing.T) {
	judge := fakeCompleter{out: "```json\n{\"passed\":true,\"reason\":\"遵循当前要求\"}\n```"}
	result, err := JudgeRecallSemantics(context.Background(), judge, RecallCase{
		ID: "C3", UserText: "详细说", Expect: RecallExpect{SemanticRules: []string{"当前要求优先"}},
	}, RecallObservation{Answer: "详细回答"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.Reason == "" {
		t.Fatalf("result=%+v", result)
	}
}

type fakeCompleter struct {
	out string
	err error
}

func (f fakeCompleter) Complete(context.Context, string, string) (string, error) {
	return f.out, f.err
}
