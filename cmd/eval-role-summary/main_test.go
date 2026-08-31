package main

import (
	"bytes"
	"strings"
	"testing"

	"deep-seeing/internal/evals"
)

func TestRenderSummaryOmitsAnswerAndJudgeReason(t *testing.T) {
	row := inputRow{Model: "fixture", Category: "identity_isolation"}
	row.Observation.CaseID = "RI1"
	row.Observation.StructuralPassed = true
	row.Rules = evals.RuleResult{Passed: true}
	row.Semantic = &evals.SemanticResult{Passed: true, Reason: "PRIVATE-JUDGE-REASON"}
	var out bytes.Buffer
	if err := renderSummary(&out, []inputRow{row}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "PRIVATE-JUDGE-REASON") {
		t.Fatalf("summary leaked judge prose: %s", out.String())
	}
	if !strings.Contains(out.String(), "PASS") {
		t.Fatalf("missing gate: %s", out.String())
	}
}
