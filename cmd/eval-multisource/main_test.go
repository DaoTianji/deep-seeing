package main

import (
	"testing"

	"deep-seeing/internal/evals"
)

func TestStructuralRulesPassedIgnoresOnlyAnswerDiagnostics(t *testing.T) {
	answerOnly := evals.RuleResult{Checks: []evals.Check{
		{Name: "case_completed", Passed: true},
		{Name: "answer_contains:结论", Passed: false},
	}}
	if !structuralRulesPassed(answerOnly) {
		t.Fatal("answer diagnostic incorrectly failed structural gate")
	}
	missingRead := evals.RuleResult{Checks: []evals.Check{{Name: "read:workspace:x", Passed: false}}}
	if structuralRulesPassed(missingRead) {
		t.Fatal("missing read incorrectly passed structural gate")
	}
}
