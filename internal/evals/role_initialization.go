package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const RoleInitializationSuiteSchemaVersion = 1

type RoleInitializationSuite struct {
	SchemaVersion int                      `json:"schema_version"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description,omitempty"`
	FixtureRoot   string                   `json:"fixture_root"`
	Cases         []RoleInitializationCase `json:"cases"`
}

type RoleInitializationCase struct {
	ID           string                   `json:"id"`
	Category     string                   `json:"category"`
	Description  string                   `json:"description"`
	Scenario     string                   `json:"scenario"`
	FixtureFiles []string                 `json:"fixture_files,omitempty"`
	Expect       RoleInitializationExpect `json:"expect"`
}

type RoleInitializationExpect struct {
	FinalStatus       string   `json:"final_status,omitempty"`
	HardErrorCodes    []string `json:"hard_error_codes,omitempty"`
	MustTraceEvidence bool     `json:"must_trace_evidence,omitempty"`
	MustNotNetwork    bool     `json:"must_not_network,omitempty"`
	MustRemainUnknown bool     `json:"must_remain_unknown,omitempty"`
}

func LoadRoleInitializationSuite(path string) (RoleInitializationSuite, error) {
	file, err := os.Open(path)
	if err != nil {
		return RoleInitializationSuite{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var suite RoleInitializationSuite
	if err := decoder.Decode(&suite); err != nil {
		return suite, fmt.Errorf("decode role initialization suite: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return suite, err
	}
	return suite, suite.Validate()
}

func (s RoleInitializationSuite) Validate() error {
	if s.SchemaVersion != RoleInitializationSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, RoleInitializationSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.FixtureRoot) == "" || len(s.Cases) != 42 {
		return fmt.Errorf("role initialization suite needs name, fixture_root and exactly 42 cases")
	}
	want := map[string]int{"research_plan": 6, "source_governance": 6, "corpus_citation": 6, "period_cutoff": 6, "critic_truth": 6, "private_professional": 6, "budget_recovery": 6}
	counts, ids := map[string]int{}, map[string]bool{}
	for _, item := range s.Cases {
		if strings.TrimSpace(item.ID) == "" || ids[item.ID] {
			return fmt.Errorf("invalid or duplicate case id %q", item.ID)
		}
		ids[item.ID] = true
		if _, ok := want[item.Category]; !ok {
			return fmt.Errorf("%s invalid category %q", item.ID, item.Category)
		}
		counts[item.Category]++
		if strings.TrimSpace(item.Description) == "" || strings.TrimSpace(item.Scenario) == "" {
			return fmt.Errorf("%s description and scenario required", item.ID)
		}
		for _, path := range item.FixtureFiles {
			if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
				return fmt.Errorf("%s unsafe fixture path", item.ID)
			}
		}
	}
	for category, expected := range want {
		if counts[category] != expected {
			return fmt.Errorf("category %s has %d cases, want %d", category, counts[category], expected)
		}
	}
	return nil
}
