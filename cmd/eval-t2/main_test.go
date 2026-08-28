package main

import (
	"encoding/json"
	"testing"
	"time"

	"deep-seeing/internal/evals"
)

func TestEvaluateStabilityFrozenGates(t *testing.T) {
	baseline := evals.StabilityBaseline{
		TargetTokensPerTurn: 100, OrdinaryP95MS: 8000, RecallP95MS: 20000,
		SemanticPassRate: 0.98, InfrastructureFailureRate: 0.01,
	}
	passing := stabilityStats{
		ValidRows: 100, RulePassed: 100, SemanticRows: 100, SemanticPassed: 98,
		Turns: 100, Tokens: 10_000,
		OrdinaryLatency: []time.Duration{7 * time.Second}, RecallLatency: []time.Duration{19 * time.Second},
	}
	if verdict := evaluateStability(baseline, passing); !verdict.Passed {
		t.Fatalf("passing verdict=%+v", verdict)
	}
	failing := passing
	failing.Infrastructure = 2
	failing.RulePassed = 99
	failing.SemanticPassed = 97
	failing.Tokens = 10_100
	failing.OrdinaryLatency = []time.Duration{9 * time.Second}
	failing.RecallLatency = []time.Duration{21 * time.Second}
	verdict := evaluateStability(baseline, failing)
	if verdict.Passed || len(verdict.Failures) != 6 {
		t.Fatalf("failing verdict=%+v", verdict)
	}
}

func TestPercentile95UsesNearestRank(t *testing.T) {
	values := make([]time.Duration, 20)
	for i := range values {
		values[i] = time.Duration(i+1) * time.Second
	}
	if got := percentile95(values); got != 19*time.Second {
		t.Fatalf("p95=%s, want 19s", got)
	}
}

func TestHardRulesExcludeLexicalAnswerChecks(t *testing.T) {
	var lexical reportEnvelope
	if err := json.Unmarshal([]byte(`{"rules":{"passed":false,"checks":[{"name":"case_completed","passed":true},{"name":"answer_contains:先给结论","passed":false},{"name":"answer_excludes:已经完成","passed":false}]}}`), &lexical); err != nil {
		t.Fatal(err)
	}
	if !hardRulesPassed(lexical) {
		t.Fatal("lexical answer checks must be handled by the semantic gate")
	}

	var structural reportEnvelope
	if err := json.Unmarshal([]byte(`{"rules":{"passed":false,"checks":[{"name":"read:episode:e1","passed":false},{"name":"answer_contains:过去","passed":true}]}}`), &structural); err != nil {
		t.Fatal(err)
	}
	if hardRulesPassed(structural) {
		t.Fatal("structural lifecycle failure must fail the hard gate")
	}

	var legacy reportEnvelope
	if err := json.Unmarshal([]byte(`{"rules":{"passed":true}}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if !hardRulesPassed(legacy) {
		t.Fatal("reports without detailed checks must retain aggregate semantics")
	}
}

func TestJudgeInfrastructureErrorClassification(t *testing.T) {
	for _, message := range []string{
		`Post "https://gateway.example/v1": context deadline exceeded`,
		"decode semantic judge verdict: unexpected end of JSON input",
	} {
		if !judgeInfrastructureError(message) {
			t.Fatalf("judge error %q should be infrastructure", message)
		}
	}
	if judgeInfrastructureError("") || judgeInfrastructureError("answer empty") {
		t.Fatal("missing user answer is a behavior failure, not judge infrastructure")
	}
}
