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
