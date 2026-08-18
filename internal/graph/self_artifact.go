package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/identity"
)

// SelfArtifactPointer is the Neo4j index for a SelfArtifact document.
type SelfArtifactPointer struct {
	ID         string
	Type       string // pattern|principle|tension|question
	Status     string
	Summary    string
	Confidence float64
	DocURI     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpsertSelfArtifact merges SelfArtifact node and (Self)-[:CONSIDERS]->.
func (s *Store) UpsertSelfArtifact(ctx context.Context, scope identity.TenantScope, a SelfArtifactPointer) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Errorf("artifact id required")
	}
	created := a.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	updated := a.UpdatedAt
	if updated.IsZero() {
		updated = created
	}
	return s.write(ctx, `
MERGE (self:Self {id: $self_id})
MERGE (a:SelfArtifact {id: $id})
SET a.type = $type,
    a.status = $status,
    a.summary = $summary,
    a.confidence = $confidence,
    a.doc_uri = $doc_uri,
    a.created_at = $created_at,
    a.updated_at = $updated_at
MERGE (self)-[:CONSIDERS]->(a)
`, map[string]any{
		"self_id":     scope.AgentID,
		"id":          a.ID,
		"type":        a.Type,
		"status":      a.Status,
		"summary":     a.Summary,
		"confidence":  a.Confidence,
		"doc_uri":     a.DocURI,
		"created_at":  created.UTC().Format(time.RFC3339),
		"updated_at":  updated.UTC().Format(time.RFC3339),
	})
}

// LinkArtifactEpisode creates SUPPORTED_BY or CHALLENGED_BY to an Episode.
func (s *Store) LinkArtifactEpisode(ctx context.Context, artifactID, episodeID, rel string) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	artifactID = strings.TrimSpace(artifactID)
	episodeID = strings.TrimSpace(episodeID)
	rel = strings.ToUpper(strings.TrimSpace(rel))
	if artifactID == "" || episodeID == "" {
		return fmt.Errorf("artifact and episode id required")
	}
	if rel != "SUPPORTED_BY" && rel != "CHALLENGED_BY" {
		return fmt.Errorf("rel must be SUPPORTED_BY or CHALLENGED_BY")
	}
	cypher := fmt.Sprintf(`
MERGE (a:SelfArtifact {id: $aid})
MERGE (e:Episode {id: $eid})
MERGE (a)-[:%s]->(e)
`, rel)
	return s.write(ctx, cypher, map[string]any{"aid": artifactID, "eid": episodeID})
}

// LinkArtifactTension creates TENSION_WITH between two SelfArtifacts.
func (s *Store) LinkArtifactTension(ctx context.Context, leftID, rightID string) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	leftID = strings.TrimSpace(leftID)
	rightID = strings.TrimSpace(rightID)
	if leftID == "" || rightID == "" {
		return fmt.Errorf("both artifact ids required")
	}
	return s.write(ctx, `
MERGE (a:SelfArtifact {id: $left})
MERGE (b:SelfArtifact {id: $right})
MERGE (a)-[:TENSION_WITH]->(b)
`, map[string]any{"left": leftID, "right": rightID})
}
