package selfmodel

import (
	"context"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

// ReflectionContext exposes only active pattern/tension cards. Bodies remain
// in the Self store and these cards are context, not historical evidence.
func (b DreamBridge) ReflectionContext(_ context.Context, scope identity.TenantScope, limit int) ([]memory.ReflectionContextItem, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if b.Store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	artifacts, err := b.Store.ListRecent(limit * 3)
	if err != nil {
		return nil, err
	}
	items := make([]memory.ReflectionContextItem, 0, limit)
	for _, artifact := range artifacts {
		if artifact.Status == StatusDeprecated || (artifact.Type != TypePattern && artifact.Type != TypeTension) {
			continue
		}
		items = append(items, memory.ReflectionContextItem{
			ID: artifact.ID, Kind: string(artifact.Type), Status: string(artifact.Status),
			Title: artifact.Title, Summary: artifact.Summary,
		})
		if len(items) == limit {
			break
		}
	}
	return items, nil
}
