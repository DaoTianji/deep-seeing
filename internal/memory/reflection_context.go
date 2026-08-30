package memory

import (
	"context"

	"deep-seeing/internal/identity"
)

// ReflectionContextItem is a thin, non-evidentiary view of current Self
// patterns and tensions. It helps reflection notice existing understanding but
// never substitutes for a read Episode.
type ReflectionContextItem struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type ReflectionContextReader interface {
	ReflectionContext(ctx context.Context, scope identity.TenantScope, limit int) ([]ReflectionContextItem, error)
}

func loadReflectionContext(ctx context.Context, reader ReflectionContextReader, scope identity.TenantScope) []ReflectionContextItem {
	if reader == nil {
		return nil
	}
	items, err := reader.ReflectionContext(ctx, scope, 20)
	if err != nil {
		return nil
	}
	return items
}
