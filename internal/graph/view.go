package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"deep-seeing/internal/identity"
)

// ViewNode is a safe, presentation-oriented graph node.
type ViewNode struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Title      string         `json:"title"`
	Subtitle   string         `json:"subtitle,omitempty"`
	Status     string         `json:"status,omitempty"`
	Anchor     string         `json:"anchor,omitempty"` // Self | Person — layout hint
	Properties map[string]any `json:"properties,omitempty"`
}

// ViewEdge is a presentation-oriented graph relationship.
type ViewEdge struct {
	ID         string         `json:"id"`
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Kind       string         `json:"kind"`
	Properties map[string]any `json:"properties,omitempty"`
}

// View is the current Self-centered memory graph shown by the room.
type View struct {
	Available bool       `json:"available"`
	Nodes     []ViewNode `json:"nodes"`
	Edges     []ViewEdge `json:"edges"`
}

// Visualization returns Self, the current Person, Bond/CALLS/KNOWS,
// and Episode pointers about either Self or the current Person.
func (s *Store) Visualization(ctx context.Context, scope identity.TenantScope, limit int) (View, error) {
	if s == nil {
		return View{Available: false, Nodes: []ViewNode{}, Edges: []ViewEdge{}}, nil
	}
	if err := scope.Validate(); err != nil {
		return View{}, err
	}
	if limit <= 0 {
		limit = 80
	}

	selfNode := ViewNode{
		ID: scope.AgentID, Kind: "Self", Title: scope.AgentID, Subtitle: "持续成长的主体",
	}
	personID := scope.PersonID()
	personNode := ViewNode{
		ID: personID, Kind: "Person", Title: displayNameFromPersonID(personID), Subtitle: "当前相识的人",
	}
	out := View{
		Available: true,
		Nodes:     []ViewNode{selfNode, personNode},
		Edges:     []ViewEdge{},
	}

	bond, err := s.GetBond(ctx, scope, personID)
	if err != nil {
		return View{}, err
	}
	if bond.RoleAtOrigin != "" {
		out.Edges = append(out.Edges, ViewEdge{
			ID: "knows:" + scope.AgentID + ":" + personID, Source: scope.AgentID, Target: personID, Kind: "KNOWS",
			Properties: map[string]any{"role_at_origin": bond.RoleAtOrigin},
		})
	}
	out.Edges = append(out.Edges, ViewEdge{
		ID: "bond:" + scope.AgentID + ":" + personID, Source: scope.AgentID, Target: personID, Kind: "BOND",
		Properties: map[string]any{
			"basics": bond.Basics, "concerns": bond.Concerns, "baseline": bond.Baseline,
			"strategy": bond.Strategy, "style": bond.Style, "boundaries": bond.Boundaries,
			"confidence": bond.Confidence, "source_episode_ids": bond.SourceEpisodeIDs,
		},
	})
	if bond.CallName != "" {
		out.Edges = append(out.Edges, ViewEdge{
			ID: "calls:" + personID + ":" + scope.AgentID, Source: personID, Target: scope.AgentID, Kind: "CALLS",
			Properties: map[string]any{"name": bond.CallName},
		})
	}

	seen := map[string]bool{}
	err = s.read(ctx, `
MATCH (e:Episode)-[:ABOUT]->(t)
WHERE e.self_id = $self_id
  AND (
    (t:Self AND t.id = $self_id)
    OR (t:Person AND t.id = $person_id)
  )
RETURN e, labels(t) AS labels, t.id AS target_id
ORDER BY coalesce(e.updated_at, e.created_at) DESC
LIMIT $limit
`, map[string]any{
		"self_id": scope.AgentID, "person_id": personID, "limit": int64(limit),
	}, func(rec *neo4j.Record) error {
		raw, _ := rec.Get("e")
		node, ok := raw.(neo4j.Node)
		if !ok {
			return nil
		}
		props := node.Props
		id := asString(props["id"])
		if id == "" {
			return nil
		}
		targetID := asString(mustGet(rec, "target_id"))
		labels := asStringSlice(mustGet(rec, "labels"))
		anchor := "Person"
		edgeTarget := personID
		for _, label := range labels {
			if label == "Self" {
				anchor = "Self"
				edgeTarget = scope.AgentID
				break
			}
		}
		if !seen[id] {
			seen[id] = true
			status := asString(props["status"])
			if status == "" {
				status = "active"
			}
			summary := strings.TrimSpace(asString(props["summary"]))
			if summary == "" {
				summary = "未写摘要的经历"
			}
			out.Nodes = append(out.Nodes, ViewNode{
				ID: id, Kind: "Episode", Title: summary,
				Subtitle: asString(props["kind"]), Status: status, Anchor: anchor,
				Properties: map[string]any{
					"kind": props["kind"], "doc_uri": props["doc_uri"], "session_id": props["session_id"],
					"created_at": props["created_at"], "updated_at": props["updated_at"],
					"valid": props["valid"], "invalid_reason": props["invalid_reason"],
					"experience_mode": props["experience_mode"],
					"about_self":      anchor == "Self",
				},
			})
		}
		out.Edges = append(out.Edges, ViewEdge{
			ID: fmt.Sprintf("about:%s:%s", id, targetID), Source: id, Target: edgeTarget, Kind: "ABOUT",
			Properties: map[string]any{"anchor": anchor},
		})
		return nil
	})
	if err != nil {
		return View{}, err
	}
	return out, nil
}

func mustGet(rec *neo4j.Record, key string) any {
	v, _ := rec.Get(key)
	return v
}
