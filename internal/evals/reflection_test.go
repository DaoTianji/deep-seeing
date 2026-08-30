package evals

import (
	"path/filepath"
	"testing"

	"deep-seeing/internal/memory"
)

func TestReflectionSuiteAndRules(t *testing.T) {
	suite, err := LoadReflectionSuite(filepath.Join("..", "..", "evals", "t3", "reflection_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 24 {
		t.Fatalf("cases=%d", len(suite.Cases))
	}
	c := suite.Cases[0]
	result := EvaluateReflectionRules(c, ReflectionObservation{CaseID: c.ID, Run: memory.ReflectionRun{NoChange: true}})
	if !result.Passed {
		t.Fatalf("checks=%+v", result.Checks)
	}
}
