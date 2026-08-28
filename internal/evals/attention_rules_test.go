package evals_test

import (
	"testing"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/evals"
	"deep-seeing/internal/observe"
)

func TestAttentionRulesKeepLifecycleAndReplacementHard(t *testing.T) {
	zero := 0
	four := 4
	result := evals.EvaluateAttentionTurnRules(evals.AttentionTurnExpect{
		RequiredTools: []string{"manage_attention"}, RequiredReads: []string{"workspace:new"},
		RequiredAttention: map[string]attention.Tier{"workspace:new": attention.Center},
		ForbiddenCenter:   []string{"workspace:old"}, MaxReads: &zero, CenterCount: &four,
		RequireExplicitReplacement: true,
	}, evals.AttentionTurnObservation{
		ToolStarts: []string{"manage_attention"}, ReadKeys: []string{"workspace:new"},
		Attention: map[string]attention.Tier{"workspace:new": attention.Center},
		AttentionDecisions: []observe.AttentionDecisionTrace{{
			Source: contextsource.Workspace, ID: "new", To: attention.Center,
		}},
	})
	if result.Passed {
		t.Fatal("structural attention failures were accepted")
	}
}

func TestAttentionRulesValidateIdleAndDismissedEvidence(t *testing.T) {
	one := 1
	result := evals.EvaluateAttentionTurnRules(evals.AttentionTurnExpect{
		RequiredReads:     []string{"episode:brief"},
		RequiredDismissed: []string{"episode:brief"},
		ForbiddenUses:     []string{"episode:brief"},
		RequiredAttention: map[string]attention.Tier{"episode:brief": attention.Center},
		RequiredIdleTurns: map[string]int{"episode:brief": 0},
		MaxReads:          &one,
	}, evals.AttentionTurnObservation{
		ReadKeys:           []string{"episode:brief"},
		DismissedKeys:      []string{"episode:brief"},
		Attention:          map[string]attention.Tier{"episode:brief": attention.Center},
		AttentionIdleTurns: map[string]int{"episode:brief": 0},
	})
	if !result.Passed {
		t.Fatalf("valid idle/dismissed observation failed: %+v", result.Checks)
	}
}
