package runtime

import (
	"strings"
	"time"
)

// DirtySignal marks that maintenance (Dream / consolidation) may be worth offering.
// P5.0 only defines the shape; Scheduler/daemon live in P7.
type DirtySignal struct {
	AgentID       string    `json:"agent_id"`
	Reason        string    `json:"reason"`
	EpisodeDelta  int       `json:"episode_delta,omitempty"`
	ProposalDelta int       `json:"proposal_delta,omitempty"`
	MarkedAt      time.Time `json:"marked_at"`
	CooldownUntil time.Time `json:"cooldown_until,omitempty"`
	BudgetLeft    int       `json:"budget_left"` // remaining maintenance offers in window
}

// DirtyPolicy holds cooldown / budget knobs.
type DirtyPolicy struct {
	Cooldown        time.Duration
	BudgetPerWindow int
}

// DefaultDirtyPolicy is a conservative starting point (not "every N episodes").
func DefaultDirtyPolicy() DirtyPolicy {
	return DirtyPolicy{Cooldown: 6 * time.Hour, BudgetPerWindow: 3}
}

// ShouldOfferMaintenance reports whether a dirty signal may fire a maintenance opportunity.
func ShouldOfferMaintenance(sig DirtySignal, now time.Time, pol DirtyPolicy) (bool, string) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if pol.Cooldown <= 0 {
		pol = DefaultDirtyPolicy()
	}
	if strings.TrimSpace(sig.Reason) == "" && sig.EpisodeDelta <= 0 && sig.ProposalDelta <= 0 {
		return false, "not dirty"
	}
	if !sig.CooldownUntil.IsZero() && now.Before(sig.CooldownUntil) {
		return false, "cooldown"
	}
	if sig.BudgetLeft <= 0 && pol.BudgetPerWindow > 0 {
		// BudgetLeft==0 with positive policy budget means exhausted; unset BudgetLeft (-0 from zero value)
		// Zero-value BudgetLeft with empty CooldownUntil: treat as "use full budget" only when MarkedAt set.
		if !sig.MarkedAt.IsZero() && sig.BudgetLeft == 0 {
			return false, "budget exhausted"
		}
	}
	return true, "opportunity"
}

// MarkDirty builds a signal and applies cooldown from policy.
func MarkDirty(agentID, reason string, episodeDelta, proposalDelta int, now time.Time, pol DirtyPolicy) DirtySignal {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if pol.Cooldown <= 0 {
		pol = DefaultDirtyPolicy()
	}
	budget := pol.BudgetPerWindow
	if budget <= 0 {
		budget = 1
	}
	return DirtySignal{
		AgentID: strings.TrimSpace(agentID), Reason: strings.TrimSpace(reason),
		EpisodeDelta: episodeDelta, ProposalDelta: proposalDelta,
		MarkedAt: now, BudgetLeft: budget,
	}
}

// ConsumeOffer records that a maintenance opportunity was offered (even if agent noops).
func ConsumeOffer(sig DirtySignal, now time.Time, pol DirtyPolicy) DirtySignal {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if pol.Cooldown <= 0 {
		pol = DefaultDirtyPolicy()
	}
	sig.BudgetLeft--
	if sig.BudgetLeft < 0 {
		sig.BudgetLeft = 0
	}
	sig.CooldownUntil = now.Add(pol.Cooldown)
	return sig
}
