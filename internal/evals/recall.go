// Package evals defines durable, implementation-independent behavior evaluations.
package evals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"deep-seeing/internal/observe"
)

const RecallSuiteSchemaVersion = 2

type RecallPolicy string

const (
	RecallRequired  RecallPolicy = "required"
	RecallForbidden RecallPolicy = "forbidden"
	RecallOptional  RecallPolicy = "optional"
)

// RecallSuite is a versioned, machine-readable collection of behavior contracts.
type RecallSuite struct {
	SchemaVersion int          `json:"schema_version"`
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	Cases         []RecallCase `json:"cases"`
}

// RecallCase specifies meaning and constraints, not exact wording or tool paths.
type RecallCase struct {
	ID          string          `json:"id"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	UserText    string          `json:"user_text"`
	Memory      []MemoryFixture `json:"memory,omitempty"`
	Expect      RecallExpect    `json:"expect"`
}

// MemoryFixture is written into an isolated Episode store before one case runs.
// Key is stable across runs even though the physical Episode ID is generated anew.
type MemoryFixture struct {
	Key     string            `json:"key"`
	Kind    string            `json:"kind,omitempty"`
	Content string            `json:"content"`
	Why     string            `json:"why,omitempty"`
	Meta    map[string]string `json:"metadata,omitempty"`
}

// RecallExpect contains only durable behavioral constraints.
type RecallExpect struct {
	Recall                    RecallPolicy `json:"recall"`
	RequiredMemoryKeys        []string     `json:"required_memory_keys,omitempty"`
	RequiredReadMemoryKeys    []string     `json:"required_read_memory_keys,omitempty"`
	RequiredUsedMemoryKeys    []string     `json:"required_used_memory_keys,omitempty"`
	ForbiddenMemoryKeys       []string     `json:"forbidden_memory_keys,omitempty"`
	EmptyResultsMustStayEmpty bool         `json:"empty_results_must_stay_empty,omitempty"`
	SemanticRules             []string     `json:"semantic_rules"`
}

// RecallObservation is the public evidence available after one run.
type RecallObservation struct {
	CaseID        string                        `json:"case_id"`
	Run           int                           `json:"run"`
	RecallMode    string                        `json:"recall_mode"`
	Searches      []observe.RecallSearchTrace   `json:"recall_searches,omitempty"`
	Reads         []observe.RecallReadTrace     `json:"recall_reads,omitempty"`
	Evidence      []observe.RecallEvidenceTrace `json:"recall_evidence,omitempty"`
	CandidateIDs  []string                      `json:"candidate_ids,omitempty"`
	CandidateKeys []string                      `json:"candidate_keys,omitempty"`
	ReadIDs       []string                      `json:"read_ids,omitempty"`
	ReadKeys      []string                      `json:"read_keys,omitempty"`
	UsedIDs       []string                      `json:"used_ids,omitempty"`
	UsedKeys      []string                      `json:"used_keys,omitempty"`
	DismissedIDs  []string                      `json:"dismissed_ids,omitempty"`
	DismissedKeys []string                      `json:"dismissed_keys,omitempty"`
	ToolStarts    []string                      `json:"tool_starts,omitempty"`
	Answer        string                        `json:"answer,omitempty"`
	Duration      time.Duration                 `json:"duration_ns,omitempty"`
	TokenUsage    observe.TokenUsageTrace       `json:"token_usage,omitempty"`
	Error         string                        `json:"error,omitempty"`
}

// Check is one deterministic assertion result.
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// RuleResult reports only facts that can be judged without another model.
type RuleResult struct {
	Passed bool    `json:"passed"`
	Checks []Check `json:"checks"`
}

// LoadRecallSuite reads a strict JSON suite. Unknown fields are rejected so typos
// cannot silently weaken an evaluation contract.
func LoadRecallSuite(path string) (RecallSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return RecallSuite{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var suite RecallSuite
	if err := dec.Decode(&suite); err != nil {
		return RecallSuite{}, fmt.Errorf("decode recall suite: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return RecallSuite{}, err
	}
	if err := suite.Validate(); err != nil {
		return RecallSuite{}, err
	}
	return suite, nil
}

func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("decode recall suite: multiple JSON values")
	}
	return fmt.Errorf("decode recall suite: %w", err)
}

// Validate checks contract integrity independently from any runtime implementation.
func (s RecallSuite) Validate() error {
	if s.SchemaVersion != RecallSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, RecallSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("suite name required")
	}
	if len(s.Cases) == 0 {
		return fmt.Errorf("at least one case required")
	}
	caseIDs := map[string]bool{}
	validCategories := map[string]bool{
		"no_recall": true, "explicit_recall": true,
		"ambiguous_recall": true, "evidence_conflict": true,
	}
	for i, c := range s.Cases {
		prefix := fmt.Sprintf("case[%d]", i)
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("%s id required", prefix)
		}
		if caseIDs[c.ID] {
			return fmt.Errorf("duplicate case id %q", c.ID)
		}
		caseIDs[c.ID] = true
		if strings.TrimSpace(c.Category) == "" || strings.TrimSpace(c.Description) == "" || strings.TrimSpace(c.UserText) == "" {
			return fmt.Errorf("%s category, description and user_text required", c.ID)
		}
		if !validCategories[c.Category] {
			return fmt.Errorf("%s invalid category %q", c.ID, c.Category)
		}
		switch c.Expect.Recall {
		case RecallRequired, RecallForbidden, RecallOptional:
		default:
			return fmt.Errorf("%s invalid recall policy %q", c.ID, c.Expect.Recall)
		}
		if len(c.Expect.SemanticRules) == 0 {
			return fmt.Errorf("%s needs at least one semantic rule", c.ID)
		}
		for _, rule := range c.Expect.SemanticRules {
			if strings.TrimSpace(rule) == "" {
				return fmt.Errorf("%s semantic rules cannot be blank", c.ID)
			}
		}
		fixtureKeys := map[string]bool{}
		for _, fixture := range c.Memory {
			if strings.TrimSpace(fixture.Key) == "" || strings.TrimSpace(fixture.Content) == "" {
				return fmt.Errorf("%s memory key and content required", c.ID)
			}
			if fixtureKeys[fixture.Key] {
				return fmt.Errorf("%s duplicate memory key %q", c.ID, fixture.Key)
			}
			switch fixture.Kind {
			case "", "event", "preference", "boundary", "state_observation", "self_note":
			default:
				return fmt.Errorf("%s memory %q has invalid kind %q", c.ID, fixture.Key, fixture.Kind)
			}
			if _, reserved := fixture.Meta["eval_key"]; reserved {
				return fmt.Errorf("%s memory %q cannot override reserved eval_key", c.ID, fixture.Key)
			}
			fixtureKeys[fixture.Key] = true
		}
		referenced := append([]string(nil), c.Expect.RequiredMemoryKeys...)
		referenced = append(referenced, c.Expect.RequiredReadMemoryKeys...)
		referenced = append(referenced, c.Expect.RequiredUsedMemoryKeys...)
		referenced = append(referenced, c.Expect.ForbiddenMemoryKeys...)
		for _, key := range referenced {
			if !fixtureKeys[key] {
				return fmt.Errorf("%s expectation references missing memory key %q", c.ID, key)
			}
		}
		if c.Expect.Recall == RecallForbidden && len(c.Expect.RequiredMemoryKeys) > 0 {
			return fmt.Errorf("%s forbids recall but requires memory", c.ID)
		}
		for _, required := range c.Expect.RequiredMemoryKeys {
			for _, forbidden := range c.Expect.ForbiddenMemoryKeys {
				if required == forbidden {
					return fmt.Errorf("%s memory key %q is both required and forbidden", c.ID, required)
				}
			}
		}
	}
	return nil
}

// EvaluateRecallRules applies deterministic assertions without judging prose.
func EvaluateRecallRules(c RecallCase, obs RecallObservation) RuleResult {
	searchCount := len(obs.Searches)
	checks := []Check{{Name: "turn_completed", Passed: strings.TrimSpace(obs.Error) == "", Detail: obs.Error}}
	switch c.Expect.Recall {
	case RecallRequired:
		checks = append(checks, Check{Name: "recall_required", Passed: searchCount > 0, Detail: fmt.Sprintf("searches=%d", searchCount)})
	case RecallForbidden:
		checks = append(checks, Check{Name: "recall_forbidden", Passed: searchCount == 0, Detail: fmt.Sprintf("searches=%d", searchCount)})
	case RecallOptional:
		checks = append(checks, Check{Name: "recall_optional", Passed: true, Detail: fmt.Sprintf("searches=%d", searchCount)})
	}
	candidates := stringSet(obs.CandidateKeys)
	for _, key := range c.Expect.RequiredMemoryKeys {
		checks = append(checks, Check{Name: "required_memory:" + key, Passed: candidates[key]})
	}
	read := stringSet(obs.ReadKeys)
	for _, key := range c.Expect.RequiredReadMemoryKeys {
		checks = append(checks, Check{Name: "required_read_memory:" + key, Passed: read[key]})
	}
	used := stringSet(obs.UsedKeys)
	for _, key := range c.Expect.RequiredUsedMemoryKeys {
		checks = append(checks, Check{Name: "required_used_memory:" + key, Passed: used[key]})
	}
	for _, key := range c.Expect.ForbiddenMemoryKeys {
		checks = append(checks, Check{Name: "forbidden_memory:" + key, Passed: !candidates[key]})
	}
	if c.Expect.EmptyResultsMustStayEmpty {
		allEmpty := searchCount > 0
		for _, search := range obs.Searches {
			if search.ResultCount != 0 || len(search.ResultIDs) != 0 {
				allEmpty = false
				break
			}
		}
		passed := !allEmpty || len(obs.CandidateIDs) == 0
		checks = append(checks, Check{Name: "empty_results_stay_empty", Passed: passed, Detail: fmt.Sprintf("all_empty=%t candidates=%d", allEmpty, len(obs.CandidateIDs))})
	}
	checks = append(checks, Check{Name: "evidence_lifecycle_valid", Passed: validEvidenceLifecycle(obs)})
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
			break
		}
	}
	return RuleResult{Passed: passed, Checks: checks}
}

func validEvidenceLifecycle(obs RecallObservation) bool {
	candidates := stringSet(obs.CandidateIDs)
	read := stringSet(obs.ReadIDs)
	decided := map[string]bool{}
	for _, event := range obs.Evidence {
		if !candidates[event.EpisodeID] || decided[event.EpisodeID] {
			return false
		}
		decided[event.EpisodeID] = true
		if event.Status == "used" && !read[event.EpisodeID] {
			return false
		}
		if event.Status != "used" && event.Status != "dismissed" {
			return false
		}
	}
	return true
}

// CandidateIDs flattens and de-duplicates search result IDs in first-seen order.
func CandidateIDs(searches []observe.RecallSearchTrace) []string {
	seen := map[string]bool{}
	var out []string
	for _, search := range searches {
		for _, id := range search.ResultIDs {
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// CategoryCounts supports stable suite inventory reporting.
func (s RecallSuite) CategoryCounts() map[string]int {
	out := map[string]int{}
	for _, c := range s.Cases {
		out[c.Category]++
	}
	return out
}

func SortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
