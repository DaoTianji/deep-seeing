package evals

import (
	"path/filepath"
	"testing"
)

func TestRoleInitializationSuiteInventory(t *testing.T) {
	suite, err := LoadRoleInitializationSuite(filepath.Join("..", "..", "evals", "t4", "role_initialization_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 42 {
		t.Fatalf("cases=%d", len(suite.Cases))
	}
	counts := map[string]int{}
	for _, item := range suite.Cases {
		counts[item.Category]++
	}
	for category, count := range counts {
		if count != 6 {
			t.Fatalf("%s=%d", category, count)
		}
	}
}
