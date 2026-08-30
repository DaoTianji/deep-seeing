package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"deep-seeing/internal/memory"
)

const ReflectionSuiteSchemaVersion = 1

type ReflectionSuite struct {
	SchemaVersion int              `json:"schema_version"`
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	Cases         []ReflectionCase `json:"cases"`
}

type ReflectionSeedFixture struct {
	Scope           string   `json:"scope"`
	Statement       string   `json:"statement"`
	SourceType      string   `json:"source_type"`
	Generated       bool     `json:"generated,omitempty"`
	ExperienceModes []string `json:"experience_modes,omitempty"`
}

type ReflectionMemoryFixture struct {
	Key            string            `json:"key"`
	Kind           string            `json:"kind,omitempty"`
	Content        string            `json:"content"`
	Why            string            `json:"why,omitempty"`
	ExperienceMode string            `json:"experience_mode,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type ReflectionExpect struct {
	AllowedActions        []string            `json:"allowed_actions"`
	RequiredReadKeys      []string            `json:"required_read_keys,omitempty"`
	RequiredEvidence      map[string][]string `json:"required_evidence,omitempty"`
	Change                string              `json:"change"` // forbidden|required|optional
	NoChange              bool                `json:"no_change,omitempty"`
	GeneratedSeedRequired bool                `json:"generated_seed_required,omitempty"`
	RollbackRequired      bool                `json:"rollback_required,omitempty"`
	SemanticRules         []string            `json:"semantic_rules"`
}

type ReflectionCase struct {
	ID          string                    `json:"id"`
	Category    string                    `json:"category"`
	Description string                    `json:"description"`
	Scenario    string                    `json:"scenario,omitempty"` // evidence|generative|rollback
	Seed        ReflectionSeedFixture     `json:"seed"`
	Memory      []ReflectionMemoryFixture `json:"memory,omitempty"`
	Expect      ReflectionExpect          `json:"expect"`
}

type RollbackTrace struct {
	OriginalMutationID string `json:"original_mutation_id"`
	RevertMutationID   string `json:"revert_mutation_id"`
	RevertsMutationID  string `json:"reverts_mutation_id"`
	OriginalRetained   bool   `json:"original_retained"`
	ContentRestored    bool   `json:"content_restored"`
	BeforeVersion      int64  `json:"before_version"`
	ChangedVersion     int64  `json:"changed_version"`
	RestoredVersion    int64  `json:"restored_version"`
}

type ReflectionObservation struct {
	CaseID             string               `json:"case_id"`
	RunNumber          int                  `json:"run_number"`
	Run                memory.ReflectionRun `json:"run"`
	CandidateKeys      []string             `json:"candidate_keys,omitempty"`
	ReadKeys           []string             `json:"read_keys,omitempty"`
	EvidenceByKey      map[string]string    `json:"evidence_by_key,omitempty"`
	ChangeCount        int                  `json:"change_count"`
	TokenUsage         memory.ChatUsage     `json:"token_usage"`
	CompensatingRevert bool                 `json:"compensating_revert,omitempty"`
	Rollback           *RollbackTrace       `json:"rollback_trace,omitempty"`
	Duration           time.Duration        `json:"duration_ns,omitempty"`
	Error              string               `json:"error,omitempty"`
}

func LoadReflectionSuite(path string) (ReflectionSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return ReflectionSuite{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var suite ReflectionSuite
	if err := dec.Decode(&suite); err != nil {
		return suite, fmt.Errorf("decode reflection suite: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return suite, err
	}
	return suite, suite.Validate()
}

func (s ReflectionSuite) Validate() error {
	if s.SchemaVersion != ReflectionSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, ReflectionSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" || len(s.Cases) != 24 {
		return fmt.Errorf("reflection suite needs a name and exactly 24 cases")
	}
	wantCategories := map[string]int{
		"no_reflection": 4, "consistent_experience": 4, "conflict": 4,
		"current_correction": 4, "source_isolation": 4, "dream_rollback": 4,
	}
	counts, ids := map[string]int{}, map[string]bool{}
	for _, c := range s.Cases {
		if strings.TrimSpace(c.ID) == "" || ids[c.ID] {
			return fmt.Errorf("invalid or duplicate case id %q", c.ID)
		}
		ids[c.ID] = true
		if _, ok := wantCategories[c.Category]; !ok {
			return fmt.Errorf("%s invalid category %q", c.ID, c.Category)
		}
		counts[c.Category]++
		if c.Scenario == "" {
			c.Scenario = "evidence"
		}
		if c.Scenario != "evidence" && c.Scenario != "generative" && c.Scenario != "rollback" {
			return fmt.Errorf("%s invalid scenario %q", c.ID, c.Scenario)
		}
		if strings.TrimSpace(c.Seed.Statement) == "" || strings.TrimSpace(c.Seed.Scope) == "" || strings.TrimSpace(c.Seed.SourceType) == "" {
			return fmt.Errorf("%s seed is incomplete", c.ID)
		}
		if c.Expect.Change != "forbidden" && c.Expect.Change != "required" && c.Expect.Change != "optional" {
			return fmt.Errorf("%s invalid change policy %q", c.ID, c.Expect.Change)
		}
		if len(c.Expect.AllowedActions) == 0 || len(c.Expect.SemanticRules) == 0 {
			return fmt.Errorf("%s needs allowed actions and semantic rules", c.ID)
		}
		keys := map[string]bool{}
		for _, fixture := range c.Memory {
			if fixture.Key == "" || fixture.Content == "" || keys[fixture.Key] {
				return fmt.Errorf("%s invalid memory fixture %q", c.ID, fixture.Key)
			}
			keys[fixture.Key] = true
		}
		for _, key := range c.Expect.RequiredReadKeys {
			if !keys[key] {
				return fmt.Errorf("%s required read references missing key %q", c.ID, key)
			}
		}
		for key := range c.Expect.RequiredEvidence {
			if !keys[key] {
				return fmt.Errorf("%s required evidence references missing key %q", c.ID, key)
			}
		}
	}
	for category, want := range wantCategories {
		if counts[category] != want {
			return fmt.Errorf("category %s has %d cases, want %d", category, counts[category], want)
		}
	}
	return nil
}

func EvaluateReflectionRules(c ReflectionCase, obs ReflectionObservation) RuleResult {
	checks := []Check{{Name: "run_completed", Passed: obs.Error == "", Detail: obs.Error}}
	actions := map[string]bool{}
	for _, decision := range obs.Run.Decisions {
		actions[string(decision.Action)] = true
	}
	allowed := map[string]bool{}
	for _, action := range c.Expect.AllowedActions {
		allowed[action] = true
	}
	actionOK := obs.Run.NoChange && allowed["no_change"]
	for action := range actions {
		if allowed[action] {
			actionOK = true
		} else {
			actionOK = false
			break
		}
	}
	if c.Scenario == "generative" || c.Scenario == "rollback" {
		actionOK = true
	}
	checks = append(checks, Check{Name: "allowed_actions", Passed: actionOK, Detail: strings.Join(mapKeys(actions), ",")})
	read := stringSet(obs.ReadKeys)
	for _, key := range c.Expect.RequiredReadKeys {
		checks = append(checks, Check{Name: "read:" + key, Passed: read[key]})
	}
	for key, states := range c.Expect.RequiredEvidence {
		got := obs.EvidenceByKey[key]
		checks = append(checks, Check{Name: "evidence:" + key, Passed: stringSet(states)[got], Detail: got})
	}
	switch c.Expect.Change {
	case "required":
		checks = append(checks, Check{Name: "change_required", Passed: obs.ChangeCount > 0})
	case "forbidden":
		checks = append(checks, Check{Name: "change_forbidden", Passed: obs.ChangeCount == 0, Detail: fmt.Sprintf("changes=%d", obs.ChangeCount)})
	}
	if c.Expect.NoChange {
		checks = append(checks, Check{Name: "no_change", Passed: obs.Run.NoChange})
	}
	if c.Expect.GeneratedSeedRequired {
		checks = append(checks, Check{Name: "generated_seed", Passed: len(obs.Run.GeneratedSeedIDs) > 0})
		checks = append(checks, Check{Name: "generated_containment", Passed: len(obs.Run.MutationIDs) == 0})
	}
	if c.Expect.RollbackRequired {
		checks = append(checks, Check{Name: "compensating_rollback", Passed: obs.CompensatingRevert})
	}
	passed := true
	for _, check := range checks {
		passed = passed && check.Passed
	}
	return RuleResult{Passed: passed, Checks: checks}
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
