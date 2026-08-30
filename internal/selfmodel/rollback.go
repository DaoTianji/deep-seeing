package selfmodel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deep-seeing/internal/identity"
)

// Deprecate is the compensating operation for an autonomously created Self
// artifact. The document and revision history remain durable.
func (s *Store) Deprecate(id, reason, actor string) (Artifact, Artifact, error) {
	if s == nil {
		return Artifact{}, Artifact{}, fmt.Errorf("self store required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Artifact{}, Artifact{}, fmt.Errorf("artifact id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, typ := range []Type{TypePattern, TypePrinciple, TypeTension, TypeQuestion} {
		path := filepath.Join(s.dir, DirName(typ), id+".md")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		before, err := parseArtifact(id, string(raw))
		if err != nil {
			return Artifact{}, Artifact{}, err
		}
		after := before
		if after.Status == StatusDeprecated {
			return before, after, nil
		}
		now := time.Now().UTC()
		after.Status, after.UpdatedAt = StatusDeprecated, now
		after.Revisions = append(after.Revisions, Revision{At: now, Summary: strings.TrimSpace(reason), Actor: strings.TrimSpace(actor)})
		if err := os.WriteFile(path, []byte(formatArtifact(after)), 0o644); err != nil {
			return Artifact{}, Artifact{}, err
		}
		return before, after, nil
	}
	return Artifact{}, Artifact{}, fmt.Errorf("artifact not found: %s", id)
}

func (b DreamBridge) RevertSelfMutation(ctx context.Context, scope identity.TenantScope, artifactID, reason string) (map[string]any, map[string]any, error) {
	if err := scope.Validate(); err != nil {
		return nil, nil, err
	}
	if b.Store == nil {
		return nil, nil, fmt.Errorf("self store required")
	}
	before, after, err := b.Store.Deprecate(artifactID, reason, "rollback")
	if err != nil {
		return nil, nil, err
	}
	if b.Graph != nil {
		_ = b.Graph.UpsertSelfArtifact(ctx, scope, ToPointer(after, b.Store.DocURI(after)))
	}
	return artifactStateMap(before), artifactStateMap(after), nil
}

func artifactStateMap(a Artifact) map[string]any {
	return map[string]any{
		"artifact_id": a.ID, "type": string(a.Type), "status": string(a.Status),
		"title": a.Title, "summary": a.Summary, "updated_at": a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
