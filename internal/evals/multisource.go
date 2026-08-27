package evals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/observe"
)

const MultiSourceSuiteSchemaVersion = 1

type MultiSourceSuite struct {
	SchemaVersion int               `json:"schema_version"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Cases         []MultiSourceCase `json:"cases"`
}

type MultiSourceCase struct {
	ID          string                 `json:"id"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	Bond        []BondFixture          `json:"bond,omitempty"`
	Scenes      []SceneFixture         `json:"scenes,omitempty"`
	Workspaces  []TaskWorkspaceFixture `json:"workspaces,omitempty"`
	Intents     []TaskIntentFixture    `json:"intents,omitempty"`
	Proposals   []ProposalFixture      `json:"proposals,omitempty"`
	Episodes    []MemoryFixture        `json:"episodes,omitempty"`
	UserText    string                 `json:"user_text"`
	Expect      MultiSourceExpect      `json:"expect"`
}

type BondFixture struct {
	Key   string `json:"key"`
	Slot  string `json:"slot"`
	Claim string `json:"claim"`
}

type SceneFixture struct {
	Key      string   `json:"key"`
	Title    string   `json:"title"`
	Keywords []string `json:"keywords"`
	Body     string   `json:"body"`
}

type ProposalFixture struct {
	Key        string `json:"key"`
	Field      string `json:"field"`
	Text       string `json:"text"`
	Rationale  string `json:"rationale,omitempty"`
	Hypothesis string `json:"hypothesis,omitempty"`
}

type MultiSourceExpect struct {
	RequiredCandidates   []string `json:"required_candidates,omitempty"`
	RequiredReads        []string `json:"required_reads,omitempty"`
	RequiredUses         []string `json:"required_uses,omitempty"`
	RequiredDismissed    []string `json:"required_dismissed,omitempty"`
	ForbiddenReads       []string `json:"forbidden_reads,omitempty"`
	ForbiddenUses        []string `json:"forbidden_uses,omitempty"`
	RequireQuestion      bool     `json:"require_question,omitempty"`
	AnswerMustContain    []string `json:"answer_must_contain,omitempty"`
	AnswerMustNotContain []string `json:"answer_must_not_contain,omitempty"`
}

type MultiSourceObservation struct {
	CaseID        string                          `json:"case_id"`
	Run           int                             `json:"run"`
	Sources       []observe.ContextSourceTrace    `json:"context_sources,omitempty"`
	Candidates    []observe.ContextCandidateTrace `json:"context_candidates,omitempty"`
	Reads         []observe.ContextReadTrace      `json:"context_reads,omitempty"`
	Uses          []observe.ContextUseTrace       `json:"context_uses,omitempty"`
	CandidateKeys []string                        `json:"candidate_keys,omitempty"`
	ReadKeys      []string                        `json:"read_keys,omitempty"`
	UsedKeys      []string                        `json:"used_keys,omitempty"`
	DismissedKeys []string                        `json:"dismissed_keys,omitempty"`
	Answer        string                          `json:"answer,omitempty"`
	Duration      time.Duration                   `json:"duration_ns,omitempty"`
	TokenUsage    observe.TokenUsageTrace         `json:"token_usage,omitempty"`
	Error         string                          `json:"error,omitempty"`
}

func LoadMultiSourceSuite(path string) (MultiSourceSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return MultiSourceSuite{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var suite MultiSourceSuite
	if err := dec.Decode(&suite); err != nil {
		return MultiSourceSuite{}, fmt.Errorf("decode multi-source suite: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return MultiSourceSuite{}, fmt.Errorf("decode multi-source suite: multiple JSON values")
		}
		return MultiSourceSuite{}, fmt.Errorf("decode multi-source suite: %w", err)
	}
	if err := suite.Validate(); err != nil {
		return MultiSourceSuite{}, err
	}
	return suite, nil
}

func (s MultiSourceSuite) Validate() error {
	if s.SchemaVersion != MultiSourceSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, MultiSourceSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" || len(s.Cases) == 0 {
		return fmt.Errorf("suite name and cases required")
	}
	ids := map[string]bool{}
	for _, c := range s.Cases {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Category) == "" ||
			strings.TrimSpace(c.Description) == "" || strings.TrimSpace(c.UserText) == "" {
			return fmt.Errorf("case id, category, description and user_text required")
		}
		if ids[c.ID] {
			return fmt.Errorf("duplicate case id %q", c.ID)
		}
		ids[c.ID] = true
		fixtures, err := multiSourceFixtureKeys(c)
		if err != nil {
			return fmt.Errorf("%s: %w", c.ID, err)
		}
		for _, key := range append(append(append([]string{}, c.Expect.RequiredCandidates...), c.Expect.RequiredReads...), append(c.Expect.RequiredUses, c.Expect.RequiredDismissed...)...) {
			if !fixtures[key] {
				return fmt.Errorf("%s expectation references missing fixture %q", c.ID, key)
			}
		}
		for _, source := range append(c.Expect.ForbiddenReads, c.Expect.ForbiddenUses...) {
			if !contextsource.ValidSource(contextsource.Source(source)) {
				return fmt.Errorf("%s expectation references invalid source %q", c.ID, source)
			}
		}
	}
	return nil
}

func multiSourceFixtureKeys(c MultiSourceCase) (map[string]bool, error) {
	keys := map[string]bool{}
	add := func(source contextsource.Source, key, body string) error {
		key = strings.TrimSpace(key)
		if key == "" || strings.TrimSpace(body) == "" {
			return fmt.Errorf("%s fixture key and body required", source)
		}
		full := string(source) + ":" + key
		if keys[full] {
			return fmt.Errorf("duplicate fixture %q", full)
		}
		keys[full] = true
		return nil
	}
	for _, f := range c.Bond {
		if err := add(contextsource.Bond, f.Key, f.Claim); err != nil {
			return nil, err
		}
	}
	for _, f := range c.Scenes {
		if len(f.Keywords) == 0 {
			return nil, fmt.Errorf("scene_norm:%s keywords required", f.Key)
		}
		if err := add(contextsource.SceneNorm, f.Key, f.Body); err != nil {
			return nil, err
		}
	}
	for _, f := range c.Workspaces {
		if err := add(contextsource.Workspace, f.Key, f.Body); err != nil {
			return nil, err
		}
	}
	for _, f := range c.Intents {
		if err := add(contextsource.Intent, f.Key, f.Title); err != nil {
			return nil, err
		}
	}
	for _, f := range c.Proposals {
		if err := add(contextsource.Proposal, f.Key, f.Text); err != nil {
			return nil, err
		}
	}
	for _, f := range c.Episodes {
		if err := add(contextsource.Episode, f.Key, f.Content); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

func EvaluateMultiSourceRules(c MultiSourceCase, obs MultiSourceObservation) RuleResult {
	checks := []Check{{Name: "case_completed", Passed: strings.TrimSpace(obs.Error) == "", Detail: obs.Error}}
	checks = appendKeyChecks(checks, "candidate:", c.Expect.RequiredCandidates, obs.CandidateKeys)
	checks = appendKeyChecks(checks, "read:", c.Expect.RequiredReads, obs.ReadKeys)
	checks = appendKeyChecks(checks, "used:", c.Expect.RequiredUses, obs.UsedKeys)
	checks = appendKeyChecks(checks, "dismissed:", c.Expect.RequiredDismissed, obs.DismissedKeys)
	for _, source := range c.Expect.ForbiddenReads {
		checks = append(checks, Check{Name: "forbid_read:" + source, Passed: !hasSourceRead(obs.Reads, contextsource.Source(source))})
	}
	for _, source := range c.Expect.ForbiddenUses {
		checks = append(checks, Check{Name: "forbid_use:" + source, Passed: !hasSourceUse(obs.Uses, contextsource.Source(source))})
	}
	if c.Expect.RequireQuestion {
		checks = append(checks, Check{Name: "answer_question", Passed: strings.Contains(obs.Answer, "?") || strings.Contains(obs.Answer, "？")})
	}
	lowerAnswer := strings.ToLower(obs.Answer)
	for _, phrase := range c.Expect.AnswerMustContain {
		checks = append(checks, Check{Name: "answer_contains:" + phrase, Passed: strings.Contains(lowerAnswer, strings.ToLower(phrase))})
	}
	for _, phrase := range c.Expect.AnswerMustNotContain {
		checks = append(checks, Check{Name: "answer_excludes:" + phrase, Passed: !strings.Contains(lowerAnswer, strings.ToLower(phrase))})
	}
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
		}
	}
	return RuleResult{Passed: passed, Checks: checks}
}

func appendKeyChecks(checks []Check, prefix string, required, actual []string) []Check {
	have := map[string]bool{}
	for _, key := range actual {
		have[key] = true
	}
	for _, key := range required {
		checks = append(checks, Check{Name: prefix + key, Passed: have[key]})
	}
	return checks
}

func hasSourceRead(events []observe.ContextReadTrace, source contextsource.Source) bool {
	for _, event := range events {
		if event.Source == source && event.OK {
			return true
		}
	}
	return false
}

func hasSourceUse(events []observe.ContextUseTrace, source contextsource.Source) bool {
	for _, event := range events {
		if event.Source == source {
			return true
		}
	}
	return false
}
