package evals_test

import (
	"testing"

	"deep-seeing/internal/evals"
)

func TestStabilityManifestReferencesVersionedCoreSuites(t *testing.T) {
	manifest, err := evals.LoadStabilityManifest("../../evals/t2/stability_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	inv, err := manifest.ValidateReferencedSuites()
	if err != nil {
		t.Fatal(err)
	}
	if inv.Cases != 28 || inv.Turns != 35 {
		t.Fatalf("inventory cases=%d turns=%d, want 28/35", inv.Cases, inv.Turns)
	}
	for _, kind := range []string{"recall", "task_context", "multisource", "attention"} {
		if inv.Kinds[kind] == 0 {
			t.Fatalf("missing suite kind %q", kind)
		}
	}
	if manifest.Baseline.TargetTokensPerTurn != 11566 || manifest.Baseline.SemanticPassRate != 0.98 {
		t.Fatalf("unexpected thresholds: %+v", manifest.Baseline)
	}
}

func TestParseCaseIDs(t *testing.T) {
	selected := evals.ParseCaseIDs(" A, B,A ,, ")
	if len(selected) != 2 || !selected["A"] || !selected["B"] {
		t.Fatalf("selected=%v", selected)
	}
	if !evals.CaseIDSelected("", "any") || evals.CaseIDSelected("A,B", "C") {
		t.Fatal("case selection semantics changed")
	}
}
