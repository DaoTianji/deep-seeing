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

// BondAwareSideQuery puts Bond recall first, then open proposals, then episode selection.
type BondAwareSideQuery struct {
	Graph     GraphBondReader   // optional
	Proposals *ProposalStore    // optional
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
		} else if text := strings.TrimSpace(bond.FormatRecall()); text != "" {
			out = append(out, Record{
				ID:       "bond:" + bond.PersonID,
				Category: CategoryPerson,
				Key:      "bond",
				Content:  text,
				Metadata: map[string]string{"kind": "bond", "about": bond.PersonID},
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
	max := limit + 2 // bond + proposals + episodes
	if len(out) > max {
		out = out[:max]
	}
	return out, nil
}
