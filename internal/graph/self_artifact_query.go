package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// ArtifactEvidence holds episode / tension links for one SelfArtifact.
type ArtifactEvidence struct {
	SupportedBy  []string
	ChallengedBy []string
	TensionWith  []string
}

// GetArtifactEvidence reads SUPPORTED_BY / CHALLENGED_BY / TENSION_WITH for an artifact.
func (s *Store) GetArtifactEvidence(ctx context.Context, artifactID string) (ArtifactEvidence, error) {
	if s == nil {
		return ArtifactEvidence{}, fmt.Errorf("neo4j: nil store")
	}
	artifactID = strings.TrimSpace(artifactID)
	if artifactID == "" {
		return ArtifactEvidence{}, fmt.Errorf("artifact id required")
	}
	var out ArtifactEvidence
	err := s.read(ctx, `
MATCH (a:SelfArtifact {id: $id})
OPTIONAL MATCH (a)-[:SUPPORTED_BY]->(sup:Episode)
OPTIONAL MATCH (a)-[:CHALLENGED_BY]->(ch:Episode)
OPTIONAL MATCH (a)-[:TENSION_WITH]-(tn:SelfArtifact)
RETURN collect(DISTINCT sup.id) AS supported,
       collect(DISTINCT ch.id) AS challenged,
       collect(DISTINCT tn.id) AS tensions
`, map[string]any{"id": artifactID}, func(rec *neo4j.Record) error {
		out.SupportedBy = stringList(recValue(rec, "supported"))
		out.ChallengedBy = stringList(recValue(rec, "challenged"))
		out.TensionWith = stringList(recValue(rec, "tensions"))
		return nil
	})
	return out, err
}

func recValue(rec *neo4j.Record, key string) any {
	v, _ := rec.Get(key)
	return v
}

func stringList(v any) []string {
	switch xs := v.(type) {
	case []any:
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(xs))
		for _, s := range xs {
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
