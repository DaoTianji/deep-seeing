package evals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"deep-seeing/internal/observe"
)

const TaskContextSuiteSchemaVersion = 1

type TaskContextSuite struct {
	SchemaVersion int               `json:"schema_version"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Cases         []TaskContextCase `json:"cases"`
}

type TaskContextCase struct {
	ID          string                 `json:"id"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	UserText    string                 `json:"user_text"`
	Workspaces  []TaskWorkspaceFixture `json:"workspaces,omitempty"`
	Intents     []TaskIntentFixture    `json:"intents,omitempty"`
	Expect      TaskContextExpect      `json:"expect"`
}

type TaskWorkspaceFixture struct {
	Key     string `json:"key"`
	Type    string `json:"type,omitempty"`
	Status  string `json:"status,omitempty"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	Body    string `json:"body"`
}

type TaskIntentFixture struct {
	Key   string `json:"key"`
	Kind  string `json:"kind,omitempty"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

type TaskContextExpect struct {
	RequiredWorkspaceKeys  []string `json:"required_workspace_keys,omitempty"`
	RequiredIntentKeys     []string `json:"required_intent_keys,omitempty"`
	ForbidContextExpansion bool     `json:"forbid_context_expansion,omitempty"`
	ForbidEpisodeSearch    bool     `json:"forbid_episode_search,omitempty"`
}

type TaskContextObservation struct {
	CaseID        string                              `json:"case_id"`
	Run           int                                 `json:"run"`
	Context       *observe.TaskContextTrace           `json:"task_context,omitempty"`
	Expansions    []observe.TaskContextExpansionTrace `json:"task_context_expansions,omitempty"`
	WorkspaceKeys []string                            `json:"expanded_workspace_keys,omitempty"`
	IntentKeys    []string                            `json:"expanded_intent_keys,omitempty"`
	Searches      []observe.RecallSearchTrace         `json:"recall_searches,omitempty"`
	Answer        string                              `json:"answer,omitempty"`
	Duration      time.Duration                       `json:"duration_ns,omitempty"`
	TokenUsage    observe.TokenUsageTrace             `json:"token_usage,omitempty"`
	Error         string                              `json:"error,omitempty"`
}

func LoadTaskContextSuite(path string) (TaskContextSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return TaskContextSuite{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var suite TaskContextSuite
	if err := dec.Decode(&suite); err != nil {
		return TaskContextSuite{}, fmt.Errorf("decode task context suite: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return TaskContextSuite{}, fmt.Errorf("decode task context suite: multiple JSON values")
		}
		return TaskContextSuite{}, fmt.Errorf("decode task context suite: %w", err)
	}
	if err := suite.Validate(); err != nil {
		return TaskContextSuite{}, err
	}
	return suite, nil
}

func (s TaskContextSuite) Validate() error {
	if s.SchemaVersion != TaskContextSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, TaskContextSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" || len(s.Cases) == 0 {
		return fmt.Errorf("suite name and cases required")
	}
	caseIDs := map[string]bool{}
	for _, c := range s.Cases {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Category) == "" || strings.TrimSpace(c.Description) == "" || strings.TrimSpace(c.UserText) == "" {
			return fmt.Errorf("case id, category, description and user_text required")
		}
		if caseIDs[c.ID] {
			return fmt.Errorf("duplicate case id %q", c.ID)
		}
		caseIDs[c.ID] = true
		workspaceKeys := map[string]bool{}
		for _, fixture := range c.Workspaces {
			if strings.TrimSpace(fixture.Key) == "" || strings.TrimSpace(fixture.Title) == "" || strings.TrimSpace(fixture.Body) == "" {
				return fmt.Errorf("%s workspace key, title and body required", c.ID)
			}
			if workspaceKeys[fixture.Key] {
				return fmt.Errorf("%s duplicate workspace key %q", c.ID, fixture.Key)
			}
			workspaceKeys[fixture.Key] = true
		}
		intentKeys := map[string]bool{}
		for _, fixture := range c.Intents {
			if strings.TrimSpace(fixture.Key) == "" || strings.TrimSpace(fixture.Title) == "" {
				return fmt.Errorf("%s intent key and title required", c.ID)
			}
			if intentKeys[fixture.Key] {
				return fmt.Errorf("%s duplicate intent key %q", c.ID, fixture.Key)
			}
			intentKeys[fixture.Key] = true
		}
		for _, key := range c.Expect.RequiredWorkspaceKeys {
			if !workspaceKeys[key] {
				return fmt.Errorf("%s expectation references missing workspace key %q", c.ID, key)
			}
		}
		for _, key := range c.Expect.RequiredIntentKeys {
			if !intentKeys[key] {
				return fmt.Errorf("%s expectation references missing intent key %q", c.ID, key)
			}
		}
		if c.Expect.ForbidContextExpansion && (len(c.Expect.RequiredWorkspaceKeys) > 0 || len(c.Expect.RequiredIntentKeys) > 0) {
			return fmt.Errorf("%s cannot forbid and require context expansion", c.ID)
		}
	}
	return nil
}

func EvaluateTaskContextRules(c TaskContextCase, obs TaskContextObservation) RuleResult {
	checks := []Check{
		{Name: "turn_completed", Passed: strings.TrimSpace(obs.Error) == "", Detail: obs.Error},
		{Name: "snapshot_present", Passed: obs.Context != nil},
	}
	workspaceKeys := stringSet(obs.WorkspaceKeys)
	for _, key := range c.Expect.RequiredWorkspaceKeys {
		checks = append(checks, Check{Name: "workspace_expanded:" + key, Passed: workspaceKeys[key]})
	}
	intentKeys := stringSet(obs.IntentKeys)
	for _, key := range c.Expect.RequiredIntentKeys {
		checks = append(checks, Check{Name: "intent_expanded:" + key, Passed: intentKeys[key]})
	}
	if c.Expect.ForbidContextExpansion {
		checks = append(checks, Check{Name: "context_expansion_forbidden", Passed: len(obs.Expansions) == 0, Detail: fmt.Sprintf("expansions=%d", len(obs.Expansions))})
	}
	if c.Expect.ForbidEpisodeSearch {
		checks = append(checks, Check{Name: "episode_search_forbidden", Passed: len(obs.Searches) == 0, Detail: fmt.Sprintf("searches=%d", len(obs.Searches))})
	}
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
		}
	}
	return RuleResult{Passed: passed, Checks: checks}
}
