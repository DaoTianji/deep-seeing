package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const LearningEffectSuiteSchemaVersion = 1

type LearningEffectSuite struct {
	SchemaVersion int                  `json:"schema_version"`
	Name          string               `json:"name"`
	Description   string               `json:"description,omitempty"`
	FixtureRoot   string               `json:"fixture_root"`
	Cases         []LearningEffectCase `json:"cases"`
}

type LearningEffectCase struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Checks      []string `json:"checks"`
	Prompt      string   `json:"prompt,omitempty"`
}

func LoadLearningEffectSuite(path string) (LearningEffectSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return LearningEffectSuite{}, err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var suite LearningEffectSuite
	if err := decoder.Decode(&suite); err != nil {
		return suite, fmt.Errorf("decode learning effect suite: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return suite, err
	}
	return suite, suite.Validate()
}

func (s LearningEffectSuite) Validate() error {
	if s.SchemaVersion != LearningEffectSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, LearningEffectSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.FixtureRoot) == "" || len(s.Cases) != 48 {
		return fmt.Errorf("learning effect suite needs name, fixture_root and exactly 48 cases")
	}
	want := map[string]int{"import_reading": 6, "whole_book": 6, "temporal_perspective": 6, "author_expression": 6, "reading_runtime": 6, "director_receipt": 6, "director_behavior": 6, "persistence_isolation": 6}
	counts, ids := map[string]int{}, map[string]bool{}
	for _, item := range s.Cases {
		if strings.TrimSpace(item.ID) == "" || ids[item.ID] {
			return fmt.Errorf("invalid or duplicate case id %q", item.ID)
		}
		ids[item.ID] = true
		if _, ok := want[item.Category]; !ok {
			return fmt.Errorf("%s invalid category %q", item.ID, item.Category)
		}
		if strings.TrimSpace(item.Description) == "" || len(item.Checks) == 0 {
			return fmt.Errorf("%s needs description and checks", item.ID)
		}
		for _, check := range item.Checks {
			if strings.TrimSpace(check) == "" {
				return fmt.Errorf("%s contains empty check", item.ID)
			}
		}
		counts[item.Category]++
	}
	for category, n := range want {
		if counts[category] != n {
			return fmt.Errorf("category %s has %d cases, want %d", category, counts[category], n)
		}
	}
	return nil
}
