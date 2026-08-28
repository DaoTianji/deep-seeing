package evals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const StabilityManifestSchemaVersion = 2

type StabilityManifest struct {
	SchemaVersion   int                       `json:"schema_version"`
	Name            string                    `json:"name"`
	Description     string                    `json:"description,omitempty"`
	Baseline        StabilityBaseline         `json:"baseline"`
	OrdinaryCaseIDs []string                  `json:"ordinary_case_ids"`
	Suites          []StabilitySuiteSelection `json:"suites"`
	FaultContracts  []string                  `json:"fault_contracts"`
	rootDir         string
}

type StabilityBaseline struct {
	AverageTokensPerTurn      float64 `json:"average_tokens_per_turn"`
	TargetTokensPerTurn       int     `json:"target_tokens_per_turn"`
	EpisodeSearchP95MS        int     `json:"episode_search_p95_ms"`
	EpisodeReadP95MS          int     `json:"episode_read_p95_ms"`
	SemanticPassRate          float64 `json:"semantic_pass_rate"`
	InfrastructureFailureRate float64 `json:"infrastructure_failure_rate"`
}

type StabilitySuiteSelection struct {
	Kind    string   `json:"kind"`
	Path    string   `json:"path"`
	CaseIDs []string `json:"case_ids"`
}

type StabilityInventory struct {
	Cases int
	Turns int
	Kinds map[string]int
}

func LoadStabilityManifest(path string) (StabilityManifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return StabilityManifest{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var manifest StabilityManifest
	if err := dec.Decode(&manifest); err != nil {
		return StabilityManifest{}, fmt.Errorf("decode stability manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return StabilityManifest{}, fmt.Errorf("decode stability manifest: multiple JSON values")
		}
		return StabilityManifest{}, fmt.Errorf("decode stability manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return StabilityManifest{}, err
	}
	manifest.rootDir = filepath.Clean(filepath.Join(filepath.Dir(path), "..", ".."))
	return manifest, nil
}

func (m StabilityManifest) Validate() error {
	if m.SchemaVersion != StabilityManifestSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", m.SchemaVersion, StabilityManifestSchemaVersion)
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Suites) == 0 {
		return fmt.Errorf("manifest name and suites required")
	}
	if m.Baseline.AverageTokensPerTurn <= 0 || m.Baseline.TargetTokensPerTurn <= 0 ||
		m.Baseline.TargetTokensPerTurn >= int(m.Baseline.AverageTokensPerTurn) {
		return fmt.Errorf("token baseline and lower target required")
	}
	if m.Baseline.EpisodeSearchP95MS <= 0 || m.Baseline.EpisodeReadP95MS <= 0 ||
		m.Baseline.SemanticPassRate <= 0 || m.Baseline.SemanticPassRate > 1 ||
		m.Baseline.InfrastructureFailureRate < 0 || m.Baseline.InfrastructureFailureRate >= 1 {
		return fmt.Errorf("invalid stability thresholds")
	}
	validKinds := map[string]bool{"recall": true, "task_context": true, "multisource": true, "attention": true}
	seenKinds := map[string]bool{}
	seenCases := map[string]bool{}
	for _, suite := range m.Suites {
		if !validKinds[suite.Kind] || seenKinds[suite.Kind] || strings.TrimSpace(suite.Path) == "" || len(suite.CaseIDs) == 0 {
			return fmt.Errorf("invalid or duplicate suite selection %q", suite.Kind)
		}
		seenKinds[suite.Kind] = true
		for _, id := range suite.CaseIDs {
			id = strings.TrimSpace(id)
			if id == "" || seenCases[id] {
				return fmt.Errorf("blank or duplicate stability case %q", id)
			}
			seenCases[id] = true
		}
	}
	if len(seenCases) < 25 || len(seenCases) > 30 {
		return fmt.Errorf("stability core needs 25-30 cases, got %d", len(seenCases))
	}
	for _, id := range m.OrdinaryCaseIDs {
		if !seenCases[id] {
			return fmt.Errorf("ordinary case %q is not selected", id)
		}
	}
	validFaults := map[string]bool{
		"source_unavailable": true, "source_error": true, "search_error": true,
		"read_error": true, "attention_unavailable": true, "step_limit": true,
		"timeout": true, "stream_interrupted": true, "session_state_reset": true,
	}
	seenFaults := map[string]bool{}
	for _, code := range m.FaultContracts {
		if !validFaults[code] || seenFaults[code] {
			return fmt.Errorf("invalid or duplicate fault contract %q", code)
		}
		seenFaults[code] = true
	}
	for code := range validFaults {
		if !seenFaults[code] {
			return fmt.Errorf("missing fault contract %q", code)
		}
	}
	return nil
}

func (m StabilityManifest) ValidateReferencedSuites() (StabilityInventory, error) {
	inv := StabilityInventory{Kinds: map[string]int{}}
	for _, selected := range m.Suites {
		available := map[string]int{}
		path := selected.Path
		if _, err := os.Stat(path); err != nil && m.rootDir != "" {
			path = filepath.Join(m.rootDir, selected.Path)
		}
		switch selected.Kind {
		case "recall":
			suite, err := LoadRecallSuite(path)
			if err != nil {
				return inv, err
			}
			for _, c := range suite.Cases {
				available[c.ID] = 1
			}
		case "task_context":
			suite, err := LoadTaskContextSuite(path)
			if err != nil {
				return inv, err
			}
			for _, c := range suite.Cases {
				available[c.ID] = len(c.Turns)
			}
		case "multisource":
			suite, err := LoadMultiSourceSuite(path)
			if err != nil {
				return inv, err
			}
			for _, c := range suite.Cases {
				available[c.ID] = 1
			}
		case "attention":
			suite, err := LoadAttentionSuite(path)
			if err != nil {
				return inv, err
			}
			for _, c := range suite.Cases {
				available[c.ID] = len(c.Turns)
			}
		}
		for _, id := range selected.CaseIDs {
			turns, ok := available[id]
			if !ok {
				return inv, fmt.Errorf("%s references unknown case %q", selected.Kind, id)
			}
			inv.Cases++
			inv.Turns += turns
			inv.Kinds[selected.Kind]++
		}
	}
	return inv, nil
}

func (m StabilityManifest) SelectedIDs(kind string) []string {
	for _, suite := range m.Suites {
		if suite.Kind == kind {
			return append([]string(nil), suite.CaseIDs...)
		}
	}
	return nil
}

func (i StabilityInventory) SortedKinds() []string {
	keys := make([]string, 0, len(i.Kinds))
	for key := range i.Kinds {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func ParseCaseIDs(raw string) map[string]bool {
	out := map[string]bool{}
	for _, value := range strings.Split(raw, ",") {
		if id := strings.TrimSpace(value); id != "" {
			out[id] = true
		}
	}
	return out
}

func CaseIDSelected(raw, id string) bool {
	selected := ParseCaseIDs(raw)
	return len(selected) == 0 || selected[id]
}
