package memory

import (
	"context"
	"fmt"
	"time"

	"deep-seeing/internal/identity"
)

type ReflectionTensionResolver interface {
	ResolveTension(ctx context.Context, scope identity.TenantScope, artifactID, summary string, sourceEpisodeIDs []string) (resolvedID string, before, after map[string]any, err error)
}

func (e *ReflectionEngine) resolveTension(ctx context.Context, scope identity.TenantScope, runID string, seed ReflectionSeed, decision ReflectionDecision, evidence []ReflectionEvidence) (string, error) {
	if e.Tensions == nil || e.Ledger == nil {
		return "", fmt.Errorf("tension resolver and ledger required")
	}
	resolvedID, before, after, err := e.Tensions.ResolveTension(ctx, scope, decision.Field, decision.SuggestedText, evidenceEpisodeIDs(evidence))
	if err != nil {
		return "", err
	}
	mutation, err := e.Ledger.Append(Mutation{
		Kind: "tension_resolution", SelfID: scope.AgentID, Field: resolvedID,
		Before: before, After: after, SourceEpisodeIDs: evidenceEpisodeIDs(evidence),
		SourceSessionIDs: nonEmpty(seed.SessionID), ReflectionSeedID: seed.ID, ReflectionRunID: runID,
		Actor: "reflection", ModelVersion: e.Model, ReasonSummary: decision.ReasonSummary, Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return mutation.ID, nil
}
