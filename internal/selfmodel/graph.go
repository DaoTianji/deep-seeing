package selfmodel

import (
	"deep-seeing/internal/graph"
)

// ToPointer builds a Neo4j pointer from a file artifact.
func ToPointer(a Artifact, docURI string) graph.SelfArtifactPointer {
	return graph.SelfArtifactPointer{
		ID: a.ID, Type: string(a.Type), Status: string(a.Status),
		Summary: a.Summary, Confidence: a.Confidence, DocURI: docURI,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}
