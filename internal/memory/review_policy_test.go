package memory_test

import (
	"testing"

	"deep-seeing/internal/memory"
)

func TestPrinciplePolicyRejectsRoleplayOnly(t *testing.T) {
	p := memory.BondProposal{Kind: memory.ProposalKindPrinciple, SuggestedText: "自由与责任绑定"}
	dec, reason := memory.PolicyFor(memory.ProposalKindPrinciple).Evaluate(p, memory.ReviewContext{
		ExperienceModes: []memory.ExperienceMode{memory.ExperienceSimulatedRoleplay},
	})
	if dec == memory.ReviewAllowAccept {
		t.Fatalf("should not allow accept: %s", reason)
	}
}

func TestSelfPatternPolicyAllowsTentative(t *testing.T) {
	p := memory.BondProposal{Kind: memory.ProposalKindSelfPattern, SuggestedText: "我在压力下容易收紧"}
	dec, reason := memory.PolicyFor(memory.ProposalKindSelfPattern).Evaluate(p, memory.ReviewContext{
		ExperienceModes: []memory.ExperienceMode{memory.ExperienceSimulatedRoleplay},
	})
	if dec != memory.ReviewAllowAccept {
		t.Fatalf("expected allow tentative: %s %s", dec, reason)
	}
}

func TestBondPolicyAllowsAppendStyle(t *testing.T) {
	p := memory.BondProposal{Kind: memory.ProposalKindBond, Field: "style", Mode: "append", SuggestedText: "更直接"}
	dec, reason := memory.PolicyFor(memory.ProposalKindBond).Evaluate(p, memory.ReviewContext{})
	if dec != memory.ReviewAllowAccept {
		t.Fatalf("expected allow: %s %s", dec, reason)
	}
}

func TestForbiddenSoulTarget(t *testing.T) {
	if err := memory.ForbiddenProposalTarget("soul"); err == nil {
		t.Fatal("expected error")
	}
}
