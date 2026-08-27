package evals

import (
	"path/filepath"
	"testing"

	"deep-seeing/internal/observe"
)

func TestTaskContextSuiteLoadsAndHasPairedContextCases(t *testing.T) {
	suite, err := LoadTaskContextSuite(filepath.Join("..", "..", "evals", "t2", "task_context_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 5 {
		t.Fatalf("cases=%d, want 5", len(suite.Cases))
	}
	var paired int
	for _, c := range suite.Cases {
		if c.Category == "context_switch" {
			paired++
		}
	}
	if paired != 2 {
		t.Fatalf("context switch cases=%d, want 2", paired)
	}
}

func TestEvaluateTaskContextRules(t *testing.T) {
	c := TaskContextCase{
		Expect: TaskContextExpect{RequiredWorkspaceKeys: []string{"target"}, ForbidEpisodeSearch: true},
	}
	pass := EvaluateTaskContextRules(c, TaskContextObservation{
		Context:       &observe.TaskContextTrace{Version: "1"},
		WorkspaceKeys: []string{"target"},
	})
	if !pass.Passed {
		t.Fatalf("expected pass: %+v", pass)
	}
	fail := EvaluateTaskContextRules(c, TaskContextObservation{
		Context:  &observe.TaskContextTrace{Version: "1"},
		Searches: []observe.RecallSearchTrace{{Query: "wrong"}},
	})
	if fail.Passed {
		t.Fatalf("expected failure: %+v", fail)
	}
}

func TestTaskContextSuiteRejectsConflictingExpectations(t *testing.T) {
	suite := TaskContextSuite{
		SchemaVersion: TaskContextSuiteSchemaVersion,
		Name:          "bad",
		Cases: []TaskContextCase{{
			ID: "x", Category: "x", Description: "x", UserText: "x",
			Workspaces: []TaskWorkspaceFixture{{Key: "w", Title: "w", Body: "w"}},
			Expect:     TaskContextExpect{RequiredWorkspaceKeys: []string{"w"}, ForbidContextExpansion: true},
		}},
	}
	if err := suite.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
