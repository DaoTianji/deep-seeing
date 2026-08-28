package evals

import (
	"path/filepath"
	"testing"

	"deep-seeing/internal/observe"
)

func TestTaskContextSuiteLoadsCompleteT23Cases(t *testing.T) {
	suite, err := LoadTaskContextSuite(filepath.Join("..", "..", "evals", "t2", "task_context_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 9 {
		t.Fatalf("cases=%d, want 9", len(suite.Cases))
	}
	categories := map[string]bool{}
	multiTurn := 0
	for _, c := range suite.Cases {
		categories[c.Category] = true
		if len(c.Turns) > 1 {
			multiTurn++
		}
	}
	for _, want := range []string{"session_continuity", "explicit_switch", "ambiguous_reference", "snapshot_overflow", "clear_focus"} {
		if !categories[want] {
			t.Fatalf("missing T2.3 category %q", want)
		}
	}
	if multiTurn < 3 {
		t.Fatalf("multi-turn cases=%d, want at least 3", multiTurn)
	}
}

func TestEvaluateTaskContextRulesAcrossFocusAndDiscovery(t *testing.T) {
	confirm := false
	c := TaskContextCase{
		Turns: []TaskContextTurn{{Expect: TaskContextExpect{
			RequiredWorkspaceKeys: []string{"target"}, RequiredWorkspaceListKeys: []string{"target"},
			FocusWorkspaceKey: "target", FocusAction: "switch", FocusCertainty: "clear",
			NeedsUserConfirmation: &confirm, ForbidEpisodeSearch: true,
		}}},
	}
	pass := EvaluateTaskContextRules(c, TaskContextObservation{
		Turns: []TaskContextTurnObservation{{
			Context: &observe.TaskContextTrace{Version: "2"}, WorkspaceKeys: []string{"target"},
			ListedWorkspaceKeys: []string{"target"}, FocusWorkspaceKey: "target",
			Focus: &observe.TaskContextFocusTrace{Action: "switch", Certainty: "clear", WorkspaceID: "wp_target"},
		}},
	})
	if !pass.Passed {
		t.Fatalf("expected pass: %+v", pass)
	}
	fail := EvaluateTaskContextRules(c, TaskContextObservation{
		Turns: []TaskContextTurnObservation{{
			Context:  &observe.TaskContextTrace{Version: "2"},
			Searches: []observe.RecallSearchTrace{{Query: "wrong"}},
		}},
	})
	if fail.Passed {
		t.Fatalf("expected failure: %+v", fail)
	}
	persistentFail := EvaluateTaskContextRules(c, TaskContextObservation{
		PersistentWrites: []string{"episode"},
		Turns: []TaskContextTurnObservation{{
			Context: &observe.TaskContextTrace{Version: "2"}, WorkspaceKeys: []string{"target"},
			ListedWorkspaceKeys: []string{"target"}, FocusWorkspaceKey: "target",
			Focus: &observe.TaskContextFocusTrace{Action: "switch", Certainty: "clear", WorkspaceID: "wp_target"},
		}},
	})
	if persistentFail.Passed {
		t.Fatalf("persistent write was accepted: %+v", persistentFail)
	}
}

func TestTaskContextSuiteRejectsConflictingExpectations(t *testing.T) {
	suite := TaskContextSuite{
		SchemaVersion: TaskContextSuiteSchemaVersion,
		Name:          "bad",
		Cases: []TaskContextCase{{
			ID: "x", Category: "x", Description: "x",
			Workspaces: []TaskWorkspaceFixture{{Key: "w", Title: "w", Body: "w"}},
			Turns: []TaskContextTurn{{
				UserText: "x",
				Expect:   TaskContextExpect{RequiredWorkspaceKeys: []string{"w"}, ForbidContextExpansion: true},
			}},
		}},
	}
	if err := suite.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAnswerRequestsConfirmationRecognizesQuestionsAndExplicitRequests(t *testing.T) {
	for _, answer := range []string{
		"你指的是红队还是蓝队？",
		"请回复“蓝队”或“红队”，我再继续。",
		"Please confirm which one you mean.",
	} {
		if !answerRequestsConfirmation(answer) {
			t.Fatalf("confirmation request not recognized: %q", answer)
		}
	}
	for _, answer := range []string{"这里存在歧义。", "需要用户确认。"} {
		if answerRequestsConfirmation(answer) {
			t.Fatalf("status statement incorrectly treated as a request: %q", answer)
		}
	}
}
