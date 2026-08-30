package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

// BondStateSnapshot is a complete restorable Bond state. A compensating
// mutation restores the content while assigning a new monotonically increasing
// version; history is never erased.
type BondStateSnapshot struct {
	SelfID           string           `json:"self_id,omitempty"`
	PersonID         string           `json:"person_id,omitempty"`
	Basics           string           `json:"basics,omitempty"`
	Concerns         string           `json:"concerns,omitempty"`
	Baseline         string           `json:"baseline,omitempty"`
	Strategy         string           `json:"strategy,omitempty"`
	Style            string           `json:"style,omitempty"`
	Boundaries       string           `json:"boundaries,omitempty"`
	Items            []graph.BondItem `json:"items,omitempty"`
	Version          int64            `json:"version"`
	StrategyCache    string           `json:"strategy_cache,omitempty"`
	StrategyCacheVer int64            `json:"strategy_cache_version,omitempty"`
	Confidence       float64          `json:"confidence,omitempty"`
	SourceEpisodeIDs []string         `json:"source_episode_ids,omitempty"`
	CallName         string           `json:"call_name,omitempty"`
	RoleAtOrigin     string           `json:"role_at_origin,omitempty"`
}

func SnapshotBond(b graph.Bond) *BondStateSnapshot {
	return &BondStateSnapshot{
		SelfID: b.SelfID, PersonID: b.PersonID, Basics: b.Basics, Concerns: b.Concerns,
		Baseline: b.Baseline, Strategy: b.Strategy, Style: b.Style, Boundaries: b.Boundaries,
		Items: append([]graph.BondItem(nil), b.Items...), Version: b.Version,
		StrategyCache: b.StrategyCache, StrategyCacheVer: b.StrategyCacheVer,
		Confidence: b.Confidence, SourceEpisodeIDs: append([]string(nil), b.SourceEpisodeIDs...),
		CallName: b.CallName, RoleAtOrigin: b.RoleAtOrigin,
	}
}

func (s *BondStateSnapshot) restoreAtVersion(version int64) graph.Bond {
	if s == nil {
		return graph.Bond{Version: version}
	}
	return graph.Bond{
		SelfID: s.SelfID, PersonID: s.PersonID, Basics: s.Basics, Concerns: s.Concerns,
		Baseline: s.Baseline, Strategy: s.Strategy, Style: s.Style, Boundaries: s.Boundaries,
		Items: append([]graph.BondItem(nil), s.Items...), Version: version,
		// Derived strategy is invalid after any SoT reversal and must be rebuilt.
		StrategyCache: "", StrategyCacheVer: 0, Confidence: s.Confidence,
		SourceEpisodeIDs: append([]string(nil), s.SourceEpisodeIDs...),
		CallName:         s.CallName, RoleAtOrigin: s.RoleAtOrigin,
	}
}

func (l *MutationLedger) Get(id string) (Mutation, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Mutation{}, fmt.Errorf("mutation id required")
	}
	items, err := l.ListRecent(1_000_000)
	if err != nil {
		return Mutation{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return Mutation{}, fmt.Errorf("mutation not found: %s", id)
}

type bondStateRestorer interface {
	GetBond(ctx context.Context, scope identity.TenantScope, personID string) (graph.Bond, error)
	PersistBondState(ctx context.Context, scope identity.TenantScope, personID string, b graph.Bond) (graph.Bond, error)
}

type selfMutationReverter interface {
	RevertSelfMutation(ctx context.Context, scope identity.TenantScope, artifactID, reason string) (before, after map[string]any, err error)
}

// RevertMutation appends a compensating mutation. If the target has changed
// since the original write, it refuses to overwrite the newer understanding.
func (d *Dreamer) RevertMutation(ctx context.Context, scope identity.TenantScope, mutationID, reason string) (Mutation, error) {
	if d == nil || d.Ledger == nil {
		return Mutation{}, fmt.Errorf("mutation ledger required")
	}
	original, err := d.Ledger.Get(mutationID)
	if err != nil {
		return Mutation{}, err
	}
	if original.RevertsMutationID != "" {
		return Mutation{}, fmt.Errorf("cannot directly revert a compensating mutation")
	}
	if strings.TrimSpace(reason) == "" {
		reason = "explicit rollback"
	}
	now := time.Now().UTC()
	switch original.Kind {
	case "bond_patch":
		restorer, ok := d.Graph.(bondStateRestorer)
		if !ok || original.BeforeBond == nil {
			return Mutation{}, fmt.Errorf("bond mutation is not restorable")
		}
		current, err := restorer.GetBond(ctx, scope, original.PersonID)
		if err != nil {
			return Mutation{}, err
		}
		if original.AfterVersion > 0 && current.Version != original.AfterVersion {
			return Mutation{}, fmt.Errorf("rollback version conflict: mutation ended at %d, current is %d", original.AfterVersion, current.Version)
		}
		restored, err := restorer.PersistBondState(ctx, scope, original.PersonID, original.BeforeBond.restoreAtVersion(current.Version+1))
		if err != nil {
			return Mutation{}, err
		}
		return d.Ledger.Append(Mutation{
			Kind: "bond_revert", SelfID: scope.AgentID, PersonID: original.PersonID, Field: original.Field,
			Before: bondFieldMap(current, original.Field), After: bondFieldMap(restored, original.Field),
			BeforeBond: SnapshotBond(current), AfterBond: SnapshotBond(restored),
			BeforeVersion: current.Version, AfterVersion: restored.Version,
			SourceEpisodeIDs: append([]string(nil), original.SourceEpisodeIDs...),
			ReflectionSeedID: original.ReflectionSeedID, ReflectionRunID: original.ReflectionRunID,
			RevertsMutationID: original.ID, Actor: "rollback", ModelVersion: d.Model,
			ReasonSummary: reason, Timestamp: now,
		})
	case "self_artifact":
		reverter, ok := d.Self.(selfMutationReverter)
		if !ok {
			return Mutation{}, fmt.Errorf("self mutation is not restorable")
		}
		before, after, err := reverter.RevertSelfMutation(ctx, scope, original.Field, reason)
		if err != nil {
			return Mutation{}, err
		}
		return d.Ledger.Append(Mutation{
			Kind: "self_revert", SelfID: scope.AgentID, Field: original.Field, Before: before, After: after,
			SourceEpisodeIDs: append([]string(nil), original.SourceEpisodeIDs...),
			ReflectionSeedID: original.ReflectionSeedID, ReflectionRunID: original.ReflectionRunID,
			RevertsMutationID: original.ID, Actor: "rollback", ModelVersion: d.Model,
			ReasonSummary: reason, Timestamp: now,
		})
	default:
		return Mutation{}, fmt.Errorf("mutation kind %q is not reversible", original.Kind)
	}
}
