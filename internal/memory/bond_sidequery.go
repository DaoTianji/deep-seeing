package memory

import (
	"context"
	"log"
	"strings"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

// GraphBondReader is the SideQuery surface over Neo4j bonds.
type GraphBondReader interface {
	GetBond(ctx context.Context, scope identity.TenantScope, personID string) (graph.Bond, error)
}

// BondAwareSideQuery puts Bond recall first, then scene bypass, open proposals, then episodes.
type BondAwareSideQuery struct {
	Graph     GraphBondReader // optional
	Scenes    *SceneStore     // optional — SceneNorm keyword bypass
	Proposals *ProposalStore  // optional
	Episodes  SideQuerySelector // optional
}

func (s *BondAwareSideQuery) SelectForTurn(ctx context.Context, scope identity.TenantScope, query string, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 5
	}
	var out []Record
	if s.Graph != nil {
		bond, err := s.Graph.GetBond(ctx, scope, scope.PersonID())
		if err != nil {
			log.Printf("sidequery bond: %v", err)
		} else {
			text, trace := graph.FormatCompactRecall(bond, query)
			text = strings.TrimSpace(text)
			if text != "" {
				meta := map[string]string{
					"kind":  "bond",
					"about": bond.PersonID,
				}
				if trace.Placeholder {
					meta["placeholder"] = "1"
				}
				if trace.StrategyOmit {
					meta["strategy_omit"] = "1"
				}
				if len(trace.Slots) > 0 {
					meta["bond_slots"] = strings.Join(trace.Slots, ",")
				}
				if len(trace.ItemIDs) > 0 {
					meta["bond_item_ids"] = strings.Join(trace.ItemIDs, ",")
				}
				out = append(out, Record{
					ID:       "bond:" + bond.PersonID,
					Category: CategoryPerson,
					Key:      "bond",
					Content:  text,
					Metadata: meta,
				})
			}
		}
	}
	if s.Scenes != nil {
		scenes, err := s.Scenes.MatchQuery(scope.PersonID(), query, SceneMaxPerTurn)
		if err != nil {
			log.Printf("sidequery scenes: %v", err)
		} else if text := FormatSceneRecall(scenes); text != "" {
			ids := make([]string, 0, len(scenes))
			for _, sc := range scenes {
				ids = append(ids, sc.ID)
			}
			out = append(out, Record{
				ID:       "scenes:" + scope.PersonID(),
				Category: CategoryPerson,
				Key:      "scene_norm",
				Content:  text,
				Metadata: map[string]string{
					"kind":      "scene_norm",
					"about":     scope.PersonID(),
					"scene_ids": strings.Join(ids, ","),
				},
			})
		}
	}
	if s.Proposals != nil {
		props, err := s.Proposals.ListOpen(ctx, scope, scope.PersonID(), 5)
		if err != nil {
			log.Printf("sidequery proposals: %v", err)
		} else if text := FormatOpenRecall(props); text != "" {
			out = append(out, Record{
				ID:       "proposals:" + scope.PersonID(),
				Category: CategoryPerson,
				Key:      "bond_proposals",
				Content:  text,
				Metadata: map[string]string{"kind": "bond_proposal", "about": scope.PersonID()},
			})
		}
	}

	epLimit := limit
	if len(out) > 0 && epLimit > 3 {
		epLimit = 3
	}
	if s.Episodes != nil {
		recs, err := s.Episodes.SelectForTurn(ctx, scope, query, epLimit)
		if err != nil {
			log.Printf("sidequery episodes: %v", err)
		} else {
			out = append(out, recs...)
		}
	}
	max := limit + 3 // bond + scenes + proposals + episodes
	if len(out) > max {
		out = out[:max]
	}
	return out, nil
}
