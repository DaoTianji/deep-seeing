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

const TaskContextSuiteSchemaVersion = 2

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
	Workspaces  []TaskWorkspaceFixture `json:"workspaces,omitempty"`
	Intents     []TaskIntentFixture    `json:"intents,omitempty"`
	Turns       []TaskContextTurn      `json:"turns"`
}

type TaskContextTurn struct {
	UserText string            `json:"user_text"`
	Expect   TaskContextExpect `json:"expect"`
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
	RequiredWorkspaceKeys     []string `json:"required_workspace_keys,omitempty"`
	RequiredIntentKeys        []string `json:"required_intent_keys,omitempty"`
	RequiredWorkspaceListKeys []string `json:"required_workspace_list_keys,omitempty"`
	RequiredIntentListKeys    []string `json:"required_intent_list_keys,omitempty"`
	SnapshotFocusWorkspaceKey string   `json:"snapshot_focus_workspace_key,omitempty"`
	SnapshotFocusIntentKey    string   `json:"snapshot_focus_intent_key,omitempty"`
	FocusWorkspaceKey         string   `json:"focus_workspace_key,omitempty"`
	FocusIntentKey            string   `json:"focus_intent_key,omitempty"`
	FocusAction               string   `json:"focus_action,omitempty"`
	FocusCertainty            string   `json:"focus_certainty,omitempty"`
	NeedsUserConfirmation     *bool    `json:"needs_user_confirmation,omitempty"`
	RequireQuestion           bool     `json:"require_question,omitempty"`
	ForbidFocusDeclaration    bool     `json:"forbid_focus_declaration,omitempty"`
	ForbidContextExpansion    bool     `json:"forbid_context_expansion,omitempty"`
	ForbidEpisodeSearch       bool     `json:"forbid_episode_search,omitempty"`
}

type TaskContextObservation struct {
	CaseID           string                       `json:"case_id"`
	Run              int                          `json:"run"`
	Turns            []TaskContextTurnObservation `json:"turns"`
	Duration         time.Duration                `json:"duration_ns,omitempty"`
	TokenUsage       observe.TokenUsageTrace      `json:"token_usage,omitempty"`
	Error            string                       `json:"error,omitempty"`
	PersistentWrites []string                     `json:"persistent_writes,omitempty"`
}

type TaskContextTurnObservation struct {
	Turn                      int                                 `json:"turn"`
	Context                   *observe.TaskContextTrace           `json:"task_context,omitempty"`
	Expansions                []observe.TaskContextExpansionTrace `json:"task_context_expansions,omitempty"`
	Focus                     *observe.TaskContextFocusTrace      `json:"task_context_focus,omitempty"`
	WorkspaceKeys             []string                            `json:"expanded_workspace_keys,omitempty"`
	IntentKeys                []string                            `json:"expanded_intent_keys,omitempty"`
	ListedWorkspaceKeys       []string                            `json:"listed_workspace_keys,omitempty"`
	ListedIntentKeys          []string                            `json:"listed_intent_keys,omitempty"`
	SnapshotFocusWorkspaceKey string                              `json:"snapshot_focus_workspace_key,omitempty"`
	SnapshotFocusIntentKey    string                              `json:"snapshot_focus_intent_key,omitempty"`
	FocusWorkspaceKey         string                              `json:"focus_workspace_key,omitempty"`
	FocusIntentKey            string                              `json:"focus_intent_key,omitempty"`
	Searches                  []observe.RecallSearchTrace         `json:"recall_searches,omitempty"`
	Answer                    string                              `json:"answer,omitempty"`
	Duration                  time.Duration                       `json:"duration_ns,omitempty"`
	TokenUsage                observe.TokenUsageTrace             `json:"token_usage,omitempty"`
	Error                     string                              `json:"error,omitempty"`
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
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Category) == "" || strings.TrimSpace(c.Description) == "" || len(c.Turns) == 0 {
			return fmt.Errorf("case id, category, description and turns required")
		}
		if caseIDs[c.ID] {
			return fmt.Errorf("duplicate case id %q", c.ID)
		}
		caseIDs[c.ID] = true
		workspaceKeys, err := validateWorkspaceFixtures(c)
		if err != nil {
			return err
		}
		intentKeys, err := validateIntentFixtures(c)
		if err != nil {
			return err
		}
		for index, turn := range c.Turns {
			if strings.TrimSpace(turn.UserText) == "" {
				return fmt.Errorf("%s turn %d user_text required", c.ID, index+1)
			}
			if err := validateTaskContextExpect(c.ID, index+1, turn.Expect, workspaceKeys, intentKeys); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateWorkspaceFixtures(c TaskContextCase) (map[string]bool, error) {
	keys := map[string]bool{}
	for _, fixture := range c.Workspaces {
		if strings.TrimSpace(fixture.Key) == "" || strings.TrimSpace(fixture.Title) == "" || strings.TrimSpace(fixture.Body) == "" {
			return nil, fmt.Errorf("%s workspace key, title and body required", c.ID)
		}
		if keys[fixture.Key] {
			return nil, fmt.Errorf("%s duplicate workspace key %q", c.ID, fixture.Key)
		}
		keys[fixture.Key] = true
	}
	return keys, nil
}

func validateIntentFixtures(c TaskContextCase) (map[string]bool, error) {
	keys := map[string]bool{}
	for _, fixture := range c.Intents {
		if strings.TrimSpace(fixture.Key) == "" || strings.TrimSpace(fixture.Title) == "" {
			return nil, fmt.Errorf("%s intent key and title required", c.ID)
		}
		if keys[fixture.Key] {
			return nil, fmt.Errorf("%s duplicate intent key %q", c.ID, fixture.Key)
		}
		keys[fixture.Key] = true
	}
	return keys, nil
}

func validateTaskContextExpect(caseID string, turn int, expect TaskContextExpect, workspaceKeys, intentKeys map[string]bool) error {
	for _, key := range append(append([]string{}, expect.RequiredWorkspaceKeys...), expect.RequiredWorkspaceListKeys...) {
		if !workspaceKeys[key] {
			return fmt.Errorf("%s turn %d expectation references missing workspace key %q", caseID, turn, key)
		}
	}
	for _, key := range append(append([]string{}, expect.RequiredIntentKeys...), expect.RequiredIntentListKeys...) {
		if !intentKeys[key] {
			return fmt.Errorf("%s turn %d expectation references missing intent key %q", caseID, turn, key)
		}
	}
	for _, key := range []string{expect.SnapshotFocusWorkspaceKey, expect.FocusWorkspaceKey} {
		if key != "" && !workspaceKeys[key] {
			return fmt.Errorf("%s turn %d focus references missing workspace key %q", caseID, turn, key)
		}
	}
	for _, key := range []string{expect.SnapshotFocusIntentKey, expect.FocusIntentKey} {
		if key != "" && !intentKeys[key] {
			return fmt.Errorf("%s turn %d focus references missing intent key %q", caseID, turn, key)
		}
	}
	if expect.ForbidContextExpansion && (len(expect.RequiredWorkspaceKeys)+len(expect.RequiredIntentKeys)+len(expect.RequiredWorkspaceListKeys)+len(expect.RequiredIntentListKeys) > 0) {
		return fmt.Errorf("%s turn %d cannot forbid and require context expansion", caseID, turn)
	}
	if expect.ForbidFocusDeclaration && (expect.FocusAction != "" || expect.FocusWorkspaceKey != "" || expect.FocusIntentKey != "" || expect.FocusCertainty != "" || expect.NeedsUserConfirmation != nil) {
		return fmt.Errorf("%s turn %d cannot forbid and require focus declaration", caseID, turn)
	}
	return nil
}

func EvaluateTaskContextRules(c TaskContextCase, obs TaskContextObservation) RuleResult {
	checks := []Check{
		{Name: "case_completed", Passed: strings.TrimSpace(obs.Error) == "", Detail: obs.Error},
		{Name: "persistent_state_unchanged", Passed: len(obs.PersistentWrites) == 0, Detail: strings.Join(obs.PersistentWrites, ",")},
	}
	if len(obs.Turns) != len(c.Turns) {
		checks = append(checks, Check{Name: "turn_count", Passed: false, Detail: fmt.Sprintf("got=%d want=%d", len(obs.Turns), len(c.Turns))})
	} else {
		checks = append(checks, Check{Name: "turn_count", Passed: true})
	}
	for i, expected := range c.Turns {
		if i >= len(obs.Turns) {
			break
		}
		checks = append(checks, evaluateTaskContextTurn(i+1, expected.Expect, obs.Turns[i])...)
	}
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
		}
	}
	return RuleResult{Passed: passed, Checks: checks}
}

func evaluateTaskContextTurn(turn int, expect TaskContextExpect, obs TaskContextTurnObservation) []Check {
	prefix := fmt.Sprintf("turn_%d:", turn)
	checks := []Check{
		{Name: prefix + "completed", Passed: strings.TrimSpace(obs.Error) == "", Detail: obs.Error},
		{Name: prefix + "snapshot_present", Passed: obs.Context != nil},
	}
	checks = appendRequiredKeys(checks, prefix+"workspace_read:", expect.RequiredWorkspaceKeys, obs.WorkspaceKeys)
	checks = appendRequiredKeys(checks, prefix+"intent_read:", expect.RequiredIntentKeys, obs.IntentKeys)
	checks = appendRequiredKeys(checks, prefix+"workspace_listed:", expect.RequiredWorkspaceListKeys, obs.ListedWorkspaceKeys)
	checks = appendRequiredKeys(checks, prefix+"intent_listed:", expect.RequiredIntentListKeys, obs.ListedIntentKeys)
	if expect.SnapshotFocusWorkspaceKey != "" {
		checks = append(checks, Check{Name: prefix + "snapshot_workspace_focus", Passed: obs.SnapshotFocusWorkspaceKey == expect.SnapshotFocusWorkspaceKey, Detail: obs.SnapshotFocusWorkspaceKey})
	}
	if expect.SnapshotFocusIntentKey != "" {
		checks = append(checks, Check{Name: prefix + "snapshot_intent_focus", Passed: obs.SnapshotFocusIntentKey == expect.SnapshotFocusIntentKey, Detail: obs.SnapshotFocusIntentKey})
	}
	if expect.ForbidFocusDeclaration {
		checks = append(checks, Check{Name: prefix + "focus_forbidden", Passed: obs.Focus == nil})
	}
	if expect.FocusAction != "" || expect.FocusWorkspaceKey != "" || expect.FocusIntentKey != "" || expect.FocusCertainty != "" || expect.NeedsUserConfirmation != nil {
		checks = append(checks, Check{Name: prefix + "focus_present", Passed: obs.Focus != nil})
		if obs.Focus != nil {
			if expect.FocusAction != "" {
				checks = append(checks, Check{Name: prefix + "focus_action", Passed: obs.Focus.Action == expect.FocusAction, Detail: obs.Focus.Action})
			}
			if expect.FocusCertainty != "" {
				checks = append(checks, Check{Name: prefix + "focus_certainty", Passed: obs.Focus.Certainty == expect.FocusCertainty, Detail: obs.Focus.Certainty})
			}
			if expect.FocusWorkspaceKey != "" {
				checks = append(checks, Check{Name: prefix + "focus_workspace", Passed: obs.FocusWorkspaceKey == expect.FocusWorkspaceKey, Detail: obs.FocusWorkspaceKey})
			}
			if expect.FocusIntentKey != "" {
				checks = append(checks, Check{Name: prefix + "focus_intent", Passed: obs.FocusIntentKey == expect.FocusIntentKey, Detail: obs.FocusIntentKey})
			}
			if expect.NeedsUserConfirmation != nil {
				checks = append(checks, Check{Name: prefix + "focus_confirmation", Passed: obs.Focus.NeedsUserConfirmation == *expect.NeedsUserConfirmation})
			}
		}
	}
	if expect.RequireQuestion {
		checks = append(checks, Check{Name: prefix + "answer_asks_question", Passed: strings.Contains(obs.Answer, "？") || strings.Contains(obs.Answer, "?")})
	}
	if expect.ForbidContextExpansion {
		checks = append(checks, Check{Name: prefix + "context_expansion_forbidden", Passed: len(obs.Expansions) == 0, Detail: fmt.Sprintf("expansions=%d", len(obs.Expansions))})
	}
	if expect.ForbidEpisodeSearch {
		checks = append(checks, Check{Name: prefix + "episode_search_forbidden", Passed: len(obs.Searches) == 0, Detail: fmt.Sprintf("searches=%d", len(obs.Searches))})
	}
	return checks
}

func appendRequiredKeys(checks []Check, prefix string, required, observed []string) []Check {
	observedSet := stringSet(observed)
	for _, key := range required {
		checks = append(checks, Check{Name: prefix + key, Passed: observedSet[key]})
	}
	return checks
}
