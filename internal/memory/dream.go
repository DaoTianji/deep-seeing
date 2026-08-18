package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

// Dreamer is an opportunity-based slow-change applicator (Phase 4).
// It never runs unless asked; it may conclude No change.
type Dreamer struct {
	Chat      *ChatClient
	Proposals *ProposalStore
	Graph     DreamGraph // optional — without graph, can only defer/reject bond
	Self      SelfDreamStore
	Ledger    *MutationLedger
	Model     string
	MinOpen   int // if >0 and CountOpen < MinOpen and force=false, skip
}

// DreamGraph is the L2 surface Dream needs for Bond.
type DreamGraph interface {
	GetBond(ctx context.Context, scope identity.TenantScope, personID string) (graph.Bond, error)
	PatchBond(ctx context.Context, scope identity.TenantScope, personID string, patch graph.BondPatch) (graph.Bond, error)
}

// SelfDreamStore applies accepted self-kind proposals (implemented by selfmodel.Store).
type SelfDreamStore interface {
	ApplySelfProposal(ctx context.Context, scope identity.TenantScope, p BondProposal) (artifactID string, after map[string]any, err error)
}

// DreamResult summarizes one dream opportunity.
type DreamResult struct {
	DreamID     string   `json:"dream_id"`
	Skipped     bool     `json:"skipped"`
	Reason      string   `json:"reason,omitempty"`
	Accepted    []string `json:"accepted,omitempty"`
	Rejected    []string `json:"rejected,omitempty"`
	Deferred    []string `json:"deferred,omitempty"`
	MutationIDs []string `json:"mutation_ids,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}

type dreamModelOut struct {
	WorthDream bool   `json:"worth_dream"`
	Notes      string `json:"notes"`
	Decisions  []struct {
		ProposalID string `json:"proposal_id"`
		Action     string `json:"action"` // accept | reject | defer
		Reason     string `json:"reason"`
	} `json:"decisions"`
}

// Run offers a dream opportunity over open proposals.
func (d *Dreamer) Run(ctx context.Context, scope identity.TenantScope, force bool) (DreamResult, error) {
	if d == nil || d.Proposals == nil {
		return DreamResult{Skipped: true, Reason: "dreamer incomplete"}, nil
	}
	if err := scope.Validate(); err != nil {
		return DreamResult{}, err
	}
	dreamID := "dream_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	open, err := d.Proposals.ListOpen(ctx, scope, "", 50)
	if err != nil {
		return DreamResult{}, err
	}
	minOpen := d.MinOpen
	if minOpen <= 0 {
		minOpen = 1
	}
	if len(open) == 0 {
		return DreamResult{DreamID: dreamID, Skipped: true, Reason: "no open proposals", Notes: "No change."}, nil
	}
	if !force && len(open) < minOpen {
		return DreamResult{DreamID: dreamID, Skipped: true, Reason: fmt.Sprintf("only %d open (< %d); not forcing", len(open), minOpen)}, nil
	}

	// Without chat: conservative — defer all (still an opportunity that did nothing irreversible)
	if d.Chat == nil || strings.TrimSpace(d.Chat.APIKey) == "" {
		return DreamResult{DreamID: dreamID, Skipped: true, Reason: "no chat client; use ApplyDecision for manual accept", Notes: "No change."}, nil
	}

	var b strings.Builder
	b.WriteString("开放提案：\n")
	for _, p := range open {
		b.WriteString(fmt.Sprintf("- id=%s kind=%s person=%s field=%s mode=%s hyp=%s text=%q rationale=%q\n",
			p.ID, NormalizeProposalKind(string(p.Kind)), p.PersonID, p.Field, p.Mode, p.Hypothesis, p.SuggestedText, p.Rationale))
	}
	system := strings.TrimSpace(`
你在为 Deep-Seeing 提供一次 Dream（巩固）机会，不是强制整理记忆。

默认可以 worth_dream=false（No change）。
仅当多条证据/提案确实值得慢变写入时，才 accept。

硬规则：
1. kind=bond：style/boundaries 只能 accept 为 append，不可整段 replace。
2. kind=self_pattern：可 accept；仅角色代入/读故事证据时最多 tentative。
3. kind=principle：无 real_interaction 证据必须 defer。
4. kind=tension：可 accept 为长期开放张力，不要假装已解决。
5. Soul 永不作为修改目标。
6. 不确定 → defer。
7. 只输出 JSON，不要 markdown。

{
  "worth_dream": false,
  "notes": "No change. 或简短说明",
  "decisions": [{"proposal_id":"prop_...","action":"accept|reject|defer","reason":"..."}]
}`)
	raw, err := d.Chat.Complete(ctx, system, b.String())
	if err != nil {
		return DreamResult{}, err
	}
	raw = stripJSONFence(raw)
	var out dreamModelOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return DreamResult{}, fmt.Errorf("parse dream json: %w", err)
	}
	result := DreamResult{DreamID: dreamID, Notes: strings.TrimSpace(out.Notes)}
	if !out.WorthDream || len(out.Decisions) == 0 {
		result.Skipped = true
		result.Reason = "model: no change"
		if result.Notes == "" {
			result.Notes = "No change."
		}
		return result, nil
	}

	byID := map[string]BondProposal{}
	for _, p := range open {
		byID[p.ID] = p
	}
	for _, dec := range out.Decisions {
		pid := strings.TrimSpace(dec.ProposalID)
		p, ok := byID[pid]
		if !ok {
			continue
		}
		action := strings.ToLower(strings.TrimSpace(dec.Action))
		switch action {
		case "accept":
			mutID, err := d.applyAccept(ctx, scope, dreamID, p, dec.Reason)
			if err != nil {
				// fail soft → defer
				_, _ = d.Proposals.Resolve(ctx, p.ID, ProposalDeferred)
				result.Deferred = append(result.Deferred, p.ID)
				continue
			}
			result.Accepted = append(result.Accepted, p.ID)
			if mutID != "" {
				result.MutationIDs = append(result.MutationIDs, mutID)
			}
		case "reject":
			if _, err := d.Proposals.Resolve(ctx, p.ID, ProposalRejected); err == nil {
				result.Rejected = append(result.Rejected, p.ID)
			}
		default:
			if _, err := d.Proposals.Resolve(ctx, p.ID, ProposalDeferred); err == nil {
				result.Deferred = append(result.Deferred, p.ID)
			}
		}
	}
	return result, nil
}

// ApplyAccept forces accept of one proposal (tests / human-assisted dream).
func (d *Dreamer) ApplyAccept(ctx context.Context, scope identity.TenantScope, proposalID, dreamID, reason string) (DreamResult, error) {
	p, err := d.Proposals.Get(ctx, proposalID)
	if err != nil {
		return DreamResult{}, err
	}
	if dreamID == "" {
		dreamID = "dream_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	}
	mutID, err := d.applyAccept(ctx, scope, dreamID, p, reason)
	if err != nil {
		return DreamResult{}, err
	}
	return DreamResult{DreamID: dreamID, Accepted: []string{p.ID}, MutationIDs: []string{mutID}}, nil
}

func (d *Dreamer) applyAccept(ctx context.Context, scope identity.TenantScope, dreamID string, p BondProposal, reason string) (mutationID string, err error) {
	kind := NormalizeProposalKind(string(p.Kind))
	decision, policyReason := PolicyFor(kind).Evaluate(p, ReviewContext{ExperienceModes: p.ExperienceModes})
	if decision != ReviewAllowAccept {
		return "", fmt.Errorf("policy %s: %s", decision, policyReason)
	}
	if kind != ProposalKindBond {
		return d.applyAcceptSelf(ctx, scope, dreamID, p, reason)
	}
	if d.Graph == nil {
		return "", fmt.Errorf("graph required to accept bond proposal")
	}
	before, err := d.Graph.GetBond(ctx, scope, p.PersonID)
	if err != nil {
		return "", err
	}
	patch := proposalToPatch(p)
	after, err := d.Graph.PatchBond(ctx, scope, p.PersonID, patch)
	if err != nil {
		return "", err
	}
	if _, err := d.Proposals.Resolve(ctx, p.ID, ProposalAccepted); err != nil {
		return "", err
	}
	if d.Ledger == nil {
		return "", nil
	}
	m, err := d.Ledger.Append(Mutation{
		Kind:             "bond_patch",
		SelfID:           scope.AgentID,
		PersonID:         p.PersonID,
		Field:            p.Field,
		Before:           bondFieldMap(before, p.Field),
		After:            bondFieldMap(after, p.Field),
		SourceSessionIDs: nonEmpty(p.SessionID),
		ProposalID:       p.ID,
		DreamID:          dreamID,
		Actor:            "dream",
		ModelVersion:     d.Model,
		ReasonSummary:    firstNonEmpty(reason, p.Rationale),
		Timestamp:        time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return m.ID, nil
}

func (d *Dreamer) applyAcceptSelf(ctx context.Context, scope identity.TenantScope, dreamID string, p BondProposal, reason string) (string, error) {
	if d.Self == nil {
		return "", fmt.Errorf("self store required to accept self proposal")
	}
	artifactID, after, err := d.Self.ApplySelfProposal(ctx, scope, p)
	if err != nil {
		return "", err
	}
	if _, err := d.Proposals.Resolve(ctx, p.ID, ProposalAccepted); err != nil {
		return "", err
	}
	if d.Ledger == nil {
		return "", nil
	}
	m, err := d.Ledger.Append(Mutation{
		Kind:             "self_artifact",
		SelfID:           scope.AgentID,
		Field:            artifactID,
		After:            after,
		SourceSessionIDs: nonEmpty(p.SessionID),
		ProposalID:       p.ID,
		DreamID:          dreamID,
		Actor:            "dream",
		ModelVersion:     d.Model,
		ReasonSummary:    firstNonEmpty(reason, p.Rationale),
		Timestamp:        time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return m.ID, nil
}

func proposalToPatch(p BondProposal) graph.BondPatch {
	patch := graph.BondPatch{}
	switch p.Field {
	case "basics":
		patch.Basics = p.SuggestedText
	case "concerns":
		patch.Concerns = p.SuggestedText
	case "baseline":
		patch.Baseline = p.SuggestedText
	case "strategy":
		patch.Strategy = p.SuggestedText
	case "style":
		patch.Style = p.SuggestedText
		patch.StyleMode = "append"
	case "boundaries":
		patch.Boundaries = p.SuggestedText
		patch.BoundMode = "append"
	}
	return patch
}

func bondFieldMap(b graph.Bond, field string) map[string]any {
	m := map[string]any{}
	switch field {
	case "basics":
		m["basics"] = b.Basics
	case "concerns":
		m["concerns"] = b.Concerns
	case "baseline":
		m["baseline"] = b.Baseline
	case "strategy":
		m["strategy"] = b.Strategy
	case "style":
		m["style"] = b.Style
	case "boundaries":
		m["boundaries"] = b.Boundaries
	default:
		m["text"] = b.FormatRecall()
	}
	return m
}

func nonEmpty(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return []string{s}
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}
