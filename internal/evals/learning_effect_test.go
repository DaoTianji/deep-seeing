package evals

import "testing"

func TestLearningEffectSuiteFrozenInventory(t *testing.T) {
	suite, err := LoadLearningEffectSuite("../../evals/t4/learning_effect_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 48 {
		t.Fatalf("cases=%d", len(suite.Cases))
	}
}
