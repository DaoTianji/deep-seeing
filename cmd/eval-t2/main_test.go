package main

import (
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
