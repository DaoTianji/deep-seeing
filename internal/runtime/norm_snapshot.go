package runtime

import (
	"context"
	"sync"
	"time"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

// NormReader is the graph surface needed to build a full person norm snapshot.
type NormReader interface {
	GetBond(ctx context.Context, scope identity.TenantScope, personID string) (graph.Bond, error)
}

// NormSnapshot is the complete Global Bond background cached for one session.
type NormSnapshot struct {
	PersonID    string    `json:"person_id"`
	BondVersion int64     `json:"bond_version"`
	Text        string    `json:"text"`
	Placeholder bool      `json:"placeholder,omitempty"`
	LoadedAt    time.Time `json:"loaded_at"`
}

// NormSnapshotCache owns the session-local norm view and its invalidation.
type NormSnapshotCache struct {
	mu     sync.Mutex
	reader NormReader
	scope  identity.TenantScope
	value  NormSnapshot
	valid  bool
}

// NewNormSnapshotCache creates an unloaded session-local cache.
func NewNormSnapshotCache(reader NormReader, scope identity.TenantScope) *NormSnapshotCache {
	return &NormSnapshotCache{reader: reader, scope: scope}
}

// Snapshot returns the cached norm, loading it once when necessary.
// A missing or failing graph becomes a safe placeholder and never blocks chat.
func (c *NormSnapshotCache) Snapshot(ctx context.Context) (NormSnapshot, error) {
	if c == nil {
		return placeholderNorm(identity.LocalCLI().PersonID()), nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.valid {
		return c.value, nil
	}
	personID := c.scope.PersonID()
	if c.reader == nil {
		c.value = placeholderNorm(personID)
		c.valid = true
		return c.value, nil
	}
	bond, err := c.reader.GetBond(ctx, c.scope, personID)
	if err != nil {
		c.value = placeholderNorm(personID)
		c.valid = true
		return c.value, err
	}
	text := graph.FormatFullNormRecall(bond)
	c.value = NormSnapshot{
		PersonID:    personID,
		BondVersion: bond.Version,
		Text:        text,
		Placeholder: text == graph.BondPlaceholder,
		LoadedAt:    time.Now().UTC(),
	}
	c.valid = true
	return c.value, nil
}

// Invalidate makes the next turn reload the full Bond.
func (c *NormSnapshotCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.valid = false
	c.mu.Unlock()
}

func placeholderNorm(personID string) NormSnapshot {
	return NormSnapshot{
		PersonID:    personID,
		Text:        graph.BondPlaceholder,
		Placeholder: true,
		LoadedAt:    time.Now().UTC(),
	}
}
