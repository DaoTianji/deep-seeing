package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"deep-seeing/internal/identity"
)

type RolePointer struct {
	ID                string
	DisplayName       string
	Kind              string
	SubjectClass      string
	Status            string
	PrivateSandbox    bool
	InstanceID        string
	InstanceStatus    string
	WorldlineID       string
	ParentWorldlineID string
	WorldlineLabel    string
	PersonID          string
	Version           int64
}

type RoleSourcePointer struct {
	ID       string
	RoleID   string
	Title    string
	Kind     string
	URL      string
	Audience string
}

type RoleEpisodePointer struct {
	ID             string
	RoleID         string
	RoleInstanceID string
	RoleSessionID  string
	WorldlineID    string
	Kind           string
	MemoryClass    string
	Summary        string
	DocURI         string
	CreatedAt      time.Time
	Status         string
}

func (s *Store) UpsertRolePointer(ctx context.Context, scope identity.TenantScope, role RolePointer) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(role.ID) == "" {
		return fmt.Errorf("role id required")
	}
	return s.write(ctx, `
MERGE (self:Self {id: $self_id})
MERGE (r:Role {id: $role_id})
SET r.display_name = $display_name, r.kind = $kind, r.subject_class = $subject_class,
    r.status = $status, r.private_sandbox = $private_sandbox, r.version = $version
MERGE (self)-[:CAN_ASSUME]->(r)
FOREACH (_ IN CASE WHEN $instance_id <> '' THEN [1] ELSE [] END |
  MERGE (ri:RoleInstance {id: $instance_id})
  SET ri.status = $instance_status, ri.person_id = $person_id
  MERGE (r)-[:HAS_INSTANCE]->(ri)
)
WITH r
OPTIONAL MATCH (r)-[:HAS_INSTANCE]->(ri:RoleInstance {id: $instance_id})
FOREACH (_ IN CASE WHEN ri IS NOT NULL AND $worldline_id <> '' THEN [1] ELSE [] END |
  MERGE (w:RoleWorldline {id: $worldline_id})
  SET w.label = $worldline_label, w.parent_worldline_id = $parent_worldline_id
  MERGE (ri)-[:HAS_WORLDLINE]->(w)
)
WITH r
OPTIONAL MATCH (r)-[:HAS_INSTANCE]->(ri:RoleInstance {id: $instance_id})
FOREACH (_ IN CASE WHEN ri IS NOT NULL AND $person_id <> '' THEN [1] ELSE [] END |
  MERGE (p:Person {id: $person_id})
  MERGE (ri)-[:RELATES_TO]->(p)
)
`, map[string]any{
		"self_id": scope.AgentID, "role_id": role.ID, "display_name": role.DisplayName,
		"kind": role.Kind, "subject_class": role.SubjectClass, "status": role.Status,
		"private_sandbox": role.PrivateSandbox, "version": role.Version,
		"instance_id": role.InstanceID, "instance_status": role.InstanceStatus,
		"worldline_id": role.WorldlineID, "parent_worldline_id": role.ParentWorldlineID,
		"worldline_label": role.WorldlineLabel, "person_id": role.PersonID,
	})
}

func (s *Store) UpsertRoleSourcePointer(ctx context.Context, source RoleSourcePointer, challenged bool) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	rel := "SUPPORTED_BY"
	if challenged {
		rel = "CHALLENGED_BY"
	}
	query := `
MATCH (r:Role {id: $role_id})
MERGE (src:Source {id: $id})
SET src.title = $title, src.kind = $kind, src.url = $url, src.audience = $audience
MERGE (r)-[:` + rel + `]->(src)
`
	return s.write(ctx, query, map[string]any{
		"role_id": source.RoleID, "id": source.ID, "title": source.Title,
		"kind": source.Kind, "url": source.URL, "audience": source.Audience,
	})
}

func (s *Store) UpsertRoleEpisodePointer(ctx context.Context, scope identity.TenantScope, ep RoleEpisodePointer) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if ep.ID == "" || ep.RoleID == "" || ep.RoleInstanceID == "" || ep.WorldlineID == "" {
		return fmt.Errorf("role episode namespace required")
	}
	created := ep.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	status := strings.TrimSpace(ep.Status)
	if status == "" {
		status = "active"
	}
	return s.write(ctx, `
MATCH (r:Role {id: $role_id})-[:HAS_INSTANCE]->(ri:RoleInstance {id: $instance_id})
MATCH (ri)-[:HAS_WORLDLINE]->(w:RoleWorldline {id: $worldline_id})
MERGE (e:Episode {id: $id})
SET e.kind = $kind, e.summary = $summary, e.doc_uri = $doc_uri,
    e.role_id = $role_id, e.role_instance_id = $instance_id,
    e.role_session_id = $session_id, e.worldline_id = $worldline_id,
    e.role_memory_class = $memory_class, e.experience_mode = 'simulated_roleplay',
    e.status = $status, e.valid = $valid, e.created_at = $created_at,
    e.updated_at = $now, e.self_id = $self_id
MERGE (ri)-[:REMEMBERS]->(e)
MERGE (w)-[:CONTAINS_MEMORY]->(e)
`, map[string]any{
		"self_id": scope.AgentID, "id": ep.ID, "role_id": ep.RoleID,
		"instance_id": ep.RoleInstanceID, "session_id": ep.RoleSessionID,
		"worldline_id": ep.WorldlineID, "kind": ep.Kind, "memory_class": ep.MemoryClass,
		"summary": ep.Summary, "doc_uri": ep.DocURI, "status": status,
		"valid": status == "active", "created_at": created.UTC().Format(time.RFC3339),
		"now": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Store) appendRoleVisualization(ctx context.Context, scope identity.TenantScope, out *View, limit int) error {
	if out == nil {
		return nil
	}
	return s.read(ctx, `
MATCH (self:Self {id: $self_id})-[ca:CAN_ASSUME]->(r:Role)
OPTIONAL MATCH (r)-[hi:HAS_INSTANCE]->(ri:RoleInstance)
OPTIONAL MATCH (ri)-[hw:HAS_WORLDLINE]->(w:RoleWorldline)
OPTIONAL MATCH (ri)-[rm:REMEMBERS]->(e:Episode)
OPTIONAL MATCH (r)-[sr:SUPPORTED_BY|CHALLENGED_BY]->(src:Source)
RETURN r, ri, w, e, src, type(sr) AS source_rel
ORDER BY r.display_name, coalesce(e.updated_at, e.created_at) DESC
LIMIT $limit
`, map[string]any{"self_id": scope.AgentID, "limit": int64(limit)}, func(rec *neo4j.Record) error {
		addGraphNode := func(raw any, kind, titleKey, subtitleKey string) string {
			node, ok := raw.(neo4j.Node)
			if !ok {
				return ""
			}
			id := asString(node.Props["id"])
			if id == "" || hasViewNode(out.Nodes, id) {
				return id
			}
			title := asString(node.Props[titleKey])
			if title == "" {
				title = id
			}
			out.Nodes = append(out.Nodes, ViewNode{
				ID: id, Kind: kind, Title: title, Subtitle: asString(node.Props[subtitleKey]),
				Status: asString(node.Props["status"]), Anchor: "Role", Properties: node.Props,
			})
			return id
		}
		roleID := addGraphNode(mustGet(rec, "r"), "Role", "display_name", "kind")
		instID := addGraphNode(mustGet(rec, "ri"), "RoleInstance", "id", "status")
		worldID := addGraphNode(mustGet(rec, "w"), "RoleWorldline", "label", "parent_worldline_id")
		episodeID := addGraphNode(mustGet(rec, "e"), "RoleEpisode", "summary", "role_memory_class")
		sourceID := addGraphNode(mustGet(rec, "src"), "Source", "title", "kind")
		appendViewEdge(out, "can_assume:"+scope.AgentID+":"+roleID, scope.AgentID, roleID, "CAN_ASSUME")
		appendViewEdge(out, "has_instance:"+roleID+":"+instID, roleID, instID, "HAS_INSTANCE")
		appendViewEdge(out, "has_worldline:"+instID+":"+worldID, instID, worldID, "HAS_WORLDLINE")
		appendViewEdge(out, "remembers:"+instID+":"+episodeID, instID, episodeID, "REMEMBERS")
		appendViewEdge(out, "contains_memory:"+worldID+":"+episodeID, worldID, episodeID, "CONTAINS_MEMORY")
		rel := asString(mustGet(rec, "source_rel"))
		appendViewEdge(out, strings.ToLower(rel)+":"+roleID+":"+sourceID, roleID, sourceID, rel)
		return nil
	})
}

func hasViewNode(nodes []ViewNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func appendViewEdge(out *View, id, source, target, kind string) {
	if source == "" || target == "" || kind == "" {
		return
	}
	for _, edge := range out.Edges {
		if edge.ID == id {
			return
		}
	}
	out.Edges = append(out.Edges, ViewEdge{ID: id, Source: source, Target: target, Kind: kind})
}
