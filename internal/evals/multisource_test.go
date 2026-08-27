package evals_test

import (
	"os"
	"path/filepath"
	"testing"

	"deep-seeing/internal/evals"
)

func TestMultiSourceSuiteAndRules(t *testing.T) {
	suite, err := evals.LoadMultiSourceSuite(filepath.Join("..", "..", "evals", "t2", "multisource_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) < 12 {
		t.Fatalf("cases=%d, want at least 12", len(suite.Cases))
	}
	c := suite.Cases[1]
	obs := evals.MultiSourceObservation{
		CaseID:        c.ID,
		CandidateKeys: append([]string(nil), c.Expect.RequiredCandidates...),
		ReadKeys:      append([]string(nil), c.Expect.RequiredReads...),
		UsedKeys:      append([]string(nil), c.Expect.RequiredUses...),
		DismissedKeys: append([]string(nil), c.Expect.RequiredDismissed...),
		Answer:        "这是纯虚构结论？",
	}
	for _, phrase := range c.Expect.AnswerMustContain {
		obs.Answer += " " + phrase
	}
	if result := evals.EvaluateMultiSourceRules(c, obs); !result.Passed {
		t.Fatalf("expected rule pass: %+v", result.Checks)
	}
}

func TestMultiSourceSuiteRejectsUnknownExpectation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	raw := `{"schema_version":1,"name":"bad","cases":[{"id":"x","category":"x","description":"x","user_text":"x","expect":{"required_reads":["episode:missing"]}}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := evals.LoadMultiSourceSuite(path); err == nil {
		t.Fatal("accepted expectation for missing fixture")
	}
}
