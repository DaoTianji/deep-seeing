package selfmodel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

// SelfGraph is the optional Neo4j surface for Dream self-accept.
type SelfGraph interface {
	UpsertSelfArtifact(ctx context.Context, scope identity.TenantScope, a graph.SelfArtifactPointer) error
	LinkArtifactEpisode(ctx context.Context, artifactID, episodeID, rel string) error
}

// ApplyProposalResult is returned when Dream accepts a self-kind proposal.
type ApplyProposalResult struct {
	Artifact Artifact
	After    map[string]any
}

// ApplyAcceptedProposal writes a SelfArtifact from an accepted proposal.
// Roleplay-only evidence is capped to tentative; principle requires real_interaction.
func (s *Store) ApplyAcceptedProposal(ctx context.Context, scope identity.TenantScope, p memory.BondProposal, g SelfGraph) (ApplyProposalResult, error) {
	if s == nil {
		return ApplyProposalResult{}, fmt.Errorf("self store required")
	}
	if err := memory.ForbiddenProposalTarget(p.Field); err != nil {
		return ApplyProposalResult{}, err
	}
	if err := memory.ForbiddenProposalTarget(p.SuggestedText); err != nil {
		return ApplyProposalResult{}, err
	}
	kind := memory.NormalizeProposalKind(string(p.Kind))
	t, err := MapProposalKindToType(string(kind))
	if err != nil {
		return ApplyProposalResult{}, err
	}
	modes := append([]memory.ExperienceMode(nil), p.ExperienceModes...)
	status := StatusTentative
	switch t {
	case TypePrinciple:
		if !CanPromoteToPrinciple(modes) {
			return ApplyProposalResult{}, fmt.Errorf("principle requires real_interaction evidence")
		}
		status = StatusClaimed
	case TypeTension:
		status = StatusObservation
	case TypePattern:
		status = MaxStatusForModes(modes)
		if status == StatusClaimed {
			status = StatusTentative // patterns stay tentative unless later claimed via P5+ flow
		}
		if len(modes) == 0 {
			status = StatusObservation
		}
	default:
		status = StatusObservation
	}
	title := strings.TrimSpace(p.Field)
	body := strings.TrimSpace(p.SuggestedText)
	if title == "" {
		title = firstLine(body, 80)
	}
	a, err := s.Create(Write{
		Type: t, Status: status, Title: title, Body: body, Summary: firstLine(body, 160),
		SourceEpisodeIDs: nil, ExperienceModes: modes, Actor: "dream",
		RevisionNote: "accepted from proposal " + p.ID,
		Confidence:   0.55,
	})
	if err != nil {
		return ApplyProposalResult{}, err
	}
	if g != nil {
		_ = g.UpsertSelfArtifact(ctx, scope, ToPointer(a, s.DocURI(a)))
		for _, eid := range a.SourceEpisodeIDs {
			_ = g.LinkArtifactEpisode(ctx, a.ID, eid, "SUPPORTED_BY")
		}
	}
	return ApplyProposalResult{
		Artifact: a,
		After: map[string]any{
			"artifact_id": a.ID, "type": string(a.Type), "status": string(a.Status),
			"title": a.Title, "summary": a.Summary, "updated_at": a.UpdatedAt.UTC().Format(time.RFC3339),
		},
	}, nil
}

// DreamBridge binds Store + optional graph for memory.Dreamer.Self.
type DreamBridge struct {
	Store *Store
	Graph SelfGraph
}

// ApplySelfProposal implements memory.SelfDreamStore.
func (b DreamBridge) ApplySelfProposal(ctx context.Context, scope identity.TenantScope, p memory.BondProposal) (string, map[string]any, error) {
	if b.Store == nil {
		return "", nil, fmt.Errorf("self store required")
	}
	res, err := b.Store.ApplyAcceptedProposal(ctx, scope, p, b.Graph)
	if err != nil {
		return "", nil, err
	}
	return res.Artifact.ID, res.After, nil
}
