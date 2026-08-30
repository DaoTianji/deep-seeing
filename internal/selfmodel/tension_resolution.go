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

// ResolveTension closes an existing tension while retaining its document and
// revision history. It is intentionally narrower than general Self mutation.
func (b DreamBridge) ResolveTension(ctx context.Context, scope identity.TenantScope, artifactID, summary string, sourceEpisodeIDs []string) (string, map[string]any, map[string]any, error) {
	if err := scope.Validate(); err != nil {
		return "", nil, nil, err
	}
	if b.Store == nil {
		return "", nil, nil, fmt.Errorf("self store required")
	}
	before, after, err := b.Store.resolveTension(artifactID, summary, sourceEpisodeIDs)
	if err != nil {
		return "", nil, nil, err
	}
	if b.Graph != nil {
		_ = b.Graph.UpsertSelfArtifact(ctx, scope, ToPointer(after, b.Store.DocURI(after)))
		for _, episodeID := range sourceEpisodeIDs {
			_ = b.Graph.LinkArtifactEpisode(ctx, after.ID, episodeID, "RESOLVED_BY")
		}
	}
	return after.ID, artifactStateMap(before), artifactStateMap(after), nil
}

func (s *Store) resolveTension(id, summary string, sourceEpisodeIDs []string) (Artifact, Artifact, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Artifact{}, Artifact{}, fmt.Errorf("tension artifact id required")
	}
	path := filepath.Join(s.dir, DirName(TypeTension), id+".md")
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, Artifact{}, err
	}
	before, err := parseArtifact(id, string(raw))
	if err != nil {
		return Artifact{}, Artifact{}, err
	}
	if before.Type != TypeTension {
		return Artifact{}, Artifact{}, fmt.Errorf("artifact %s is not a tension", id)
	}
	after := before
	now := time.Now().UTC()
	after.Status, after.UpdatedAt = StatusDeprecated, now
	after.SourceEpisodeIDs = uniqueArtifactSources(after.SourceEpisodeIDs, sourceEpisodeIDs)
	after.Revisions = append(after.Revisions, Revision{At: now, Actor: "reflection", Summary: "resolved: " + strings.TrimSpace(summary)})
	if err := os.WriteFile(path, []byte(formatArtifact(after)), 0o644); err != nil {
		return Artifact{}, Artifact{}, err
	}
	return before, after, nil
}

func uniqueArtifactSources(existing, incoming []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range [][]string{existing, incoming} {
		for _, id := range group {
			id = strings.TrimSpace(id)
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}
