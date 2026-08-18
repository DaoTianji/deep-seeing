package memory

import (
	"fmt"
	"strings"
)

// ReviewDecision is the policy outcome for one proposal.
type ReviewDecision string

const (
	ReviewAllowAccept ReviewDecision = "allow_accept"
	ReviewForceDefer  ReviewDecision = "force_defer"
	ReviewForceReject ReviewDecision = "force_reject"
)

// ReviewContext carries evidence hints for Self policies.
type ReviewContext struct {
	ExperienceModes []ExperienceMode
}

// ReviewPolicy decides whether Dream may accept a proposal of a given kind.
type ReviewPolicy interface {
	Kind() ProposalKind
	Evaluate(p BondProposal, ctx ReviewContext) (ReviewDecision, string)
}

// PolicyFor returns the policy for a proposal kind.
func PolicyFor(kind ProposalKind) ReviewPolicy {
	switch NormalizeProposalKind(string(kind)) {
	case ProposalKindSelfPattern:
		return SelfPatternReviewPolicy{}
	case ProposalKindPrinciple:
		return PrincipleReviewPolicy{}
	case ProposalKindTension:
		return TensionReviewPolicy{}
	default:
		return BondReviewPolicy{}
	}
}

// BondReviewPolicy encodes existing Bond Dream discipline.
type BondReviewPolicy struct{}

func (BondReviewPolicy) Kind() ProposalKind { return ProposalKindBond }

func (BondReviewPolicy) Evaluate(p BondProposal, _ ReviewContext) (ReviewDecision, string) {
	field := normalizeProposalField(p.Field)
	if field == "" {
		return ReviewForceDefer, "bond field required"
	}
	if (field == "style" || field == "boundaries") && strings.EqualFold(p.Mode, "replace") {
		return ReviewForceDefer, "high-threshold fields cannot replace"
	}
	return ReviewAllowAccept, ""
}

// SelfPatternReviewPolicy allows accept as tentative/observation; roleplay-only cannot claim.
type SelfPatternReviewPolicy struct{}

func (SelfPatternReviewPolicy) Kind() ProposalKind { return ProposalKindSelfPattern }

func (SelfPatternReviewPolicy) Evaluate(p BondProposal, ctx ReviewContext) (ReviewDecision, string) {
	if strings.TrimSpace(p.SuggestedText) == "" {
		return ReviewForceDefer, "empty pattern text"
	}
	if err := ForbiddenProposalTarget(p.Field); err != nil {
		return ReviewForceReject, err.Error()
	}
	modes := ctx.ExperienceModes
	if len(modes) == 0 {
		modes = p.ExperienceModes
	}
	if onlyWeakExperienceModes(modes) {
		return ReviewAllowAccept, "roleplay/story-only → tentative pattern at most"
	}
	return ReviewAllowAccept, ""
}

// PrincipleReviewPolicy is strictest.
type PrincipleReviewPolicy struct{}

func (PrincipleReviewPolicy) Kind() ProposalKind { return ProposalKindPrinciple }

func (PrincipleReviewPolicy) Evaluate(p BondProposal, ctx ReviewContext) (ReviewDecision, string) {
	if strings.TrimSpace(p.SuggestedText) == "" {
		return ReviewForceDefer, "empty principle text"
	}
	modes := ctx.ExperienceModes
	if len(modes) == 0 {
		modes = p.ExperienceModes
	}
	if !modesCanPromotePrinciple(modes) {
		return ReviewForceDefer, "principle requires real_interaction evidence; roleplay/story alone insufficient"
	}
	return ReviewAllowAccept, ""
}

// TensionReviewPolicy allows creating long-lived open tensions (not auto-resolving them away).
type TensionReviewPolicy struct{}

func (TensionReviewPolicy) Kind() ProposalKind { return ProposalKindTension }

func (TensionReviewPolicy) Evaluate(p BondProposal, _ ReviewContext) (ReviewDecision, string) {
	if strings.TrimSpace(p.SuggestedText) == "" {
		return ReviewForceDefer, "empty tension text"
	}
	return ReviewAllowAccept, "tension remains open as SelfArtifact"
}

func modesCanPromotePrinciple(modes []ExperienceMode) bool {
	for _, m := range modes {
		if NormalizeExperienceMode(string(m)) == ExperienceRealInteraction {
			return true
		}
	}
	return false
}

func onlyWeakExperienceModes(modes []ExperienceMode) bool {
	if len(modes) == 0 {
		return false
	}
	for _, m := range modes {
		nm := NormalizeExperienceMode(string(m))
		if nm != ExperienceSimulatedRoleplay && nm != ExperienceStoryReading {
			return false
		}
	}
	return true
}

// ForbiddenProposalTarget ensures Soul is never a proposal target.
func ForbiddenProposalTarget(target string) error {
	t := strings.ToLower(strings.TrimSpace(target))
	if t == "soul" || t == "soul.md" || strings.Contains(t, "soul/") {
		return fmt.Errorf("Soul cannot be modified via proposals")
	}
	return nil
}
