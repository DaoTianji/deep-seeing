package graph

import (
	"context"
	"fmt"
)

// EnsureSchema creates uniqueness constraints for Self / Person / Episode.
func (s *Store) EnsureSchema(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	stmts := []string{
		`CREATE CONSTRAINT self_id IF NOT EXISTS FOR (n:Self) REQUIRE n.id IS UNIQUE`,
		`CREATE CONSTRAINT person_id IF NOT EXISTS FOR (n:Person) REQUIRE n.id IS UNIQUE`,
		`CREATE CONSTRAINT episode_id IF NOT EXISTS FOR (n:Episode) REQUIRE n.id IS UNIQUE`,
		`CREATE CONSTRAINT self_artifact_id IF NOT EXISTS FOR (n:SelfArtifact) REQUIRE n.id IS UNIQUE`,
	}
	for _, cypher := range stmts {
		if err := s.write(ctx, cypher, nil); err != nil {
			return fmt.Errorf("ensure schema: %w", err)
		}
	}
	return nil
}
