package main

import (
	"strings"
	"testing"

	"deep-seeing/internal/evals"
)

func TestAttentionInfrastructureErrorSeparatesTurnFailures(t *testing.T) {
	got := attentionInfrastructureError([]evals.AttentionTurnObservation{
		{Turn: 1},
		{Turn: 2, InfrastructureError: "stream reset"},
		{Turn: 3, Error: "agent stream: timeout"},
	})
	for _, want := range []string{"turn 2: stream reset", "turn 3: agent stream: timeout"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if clean := attentionInfrastructureError([]evals.AttentionTurnObservation{{Turn: 1}}); clean != "" {
		t.Fatalf("clean run classified as infrastructure failure: %q", clean)
	}
}
