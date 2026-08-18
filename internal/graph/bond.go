package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/origin"
)

// EnsureOriginSeed merges Self / Person / KNOWS / empty BOND.
// Only weak prior: KNOWS.role_at_origin. Never seeds trust=high or mentor role.
func (s *Store) EnsureOriginSeed(ctx context.Context, scope identity.TenantScope, roleAtOrigin string) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if roleAtOrigin == "" {
		roleAtOrigin = origin.RoleAtOrigin
	}
	personID := scope.PersonID()
	now := time.Now().UTC().Format(time.RFC3339)
	cypher := `
MERGE (self:Self {id: $self_id})
MERGE (p:Person {id: $person_id})
ON CREATE SET p.display_name = $display_name
MERGE (self)-[k:KNOWS]->(p)
ON CREATE SET k.role_at_origin = $role_at_origin, k.created_at = $now
ON MATCH SET k.role_at_origin = coalesce(k.role_at_origin, $role_at_origin)
MERGE (self)-[b:BOND]->(p)
ON CREATE SET
  b.basics = '',
  b.concerns = '',
  b.baseline = '',
  b.strategy = '',
  b.style = '',
  b.boundaries = '',
  b.confidence = 0.0,
  b.source_episode_ids = [],
  b.created_at = $now
`
	return s.write(ctx, cypher, map[string]any{
		"self_id":        scope.AgentID,
		"person_id":      personID,
		"display_name":   displayNameFromPersonID(personID),
		"role_at_origin": roleAtOrigin,
		"now":            now,
	})
}

// GetBond loads BOND + optional CALLS + KNOWS.role_at_origin for a person.
func (s *Store) GetBond(ctx context.Context, scope identity.TenantScope, personID string) (Bond, error) {
	if s == nil {
		return Bond{}, fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return Bond{}, err
	}
	personID = normalizePersonID(scope, personID)
	var out Bond
	found := false
	err := s.read(ctx, `
MATCH (self:Self {id: $self_id})-[b:BOND]->(p:Person {id: $person_id})
OPTIONAL MATCH (self)-[k:KNOWS]->(p)
OPTIONAL MATCH (p)-[c:CALLS]->(self)
RETURN b, k.role_at_origin AS role_at_origin, c.name AS call_name
LIMIT 1
`, map[string]any{
		"self_id":   scope.AgentID,
		"person_id": personID,
	}, func(rec *neo4j.Record) error {
		found = true
		raw, _ := rec.Get("b")
		rel, _ := raw.(neo4j.Relationship)
		props := map[string]any{}
		if rel.Props != nil {
			props = rel.Props
		}
		role, _ := rec.Get("role_at_origin")
		call, _ := rec.Get("call_name")
		out = Bond{
			SelfID:           scope.AgentID,
			PersonID:         personID,
			Basics:           asString(props["basics"]),
			Concerns:         asString(props["concerns"]),
			Baseline:         asString(props["baseline"]),
			Strategy:         asString(props["strategy"]),
			Style:            asString(props["style"]),
			Boundaries:       asString(props["boundaries"]),
			Confidence:       asFloat(props["confidence"]),
			LastConfirmedAt:  asTime(props["last_confirmed_at"]),
			SourceEpisodeIDs: asStringSlice(props["source_episode_ids"]),
			RoleAtOrigin:     asString(role),
			CallName:         asString(call),
		}
		return nil
	})
	if err != nil {
		return Bond{}, err
	}
	if !found {
		return Bond{SelfID: scope.AgentID, PersonID: personID}, nil
	}
	return out, nil
}

// PatchBond applies a subject patch with high-threshold append discipline.
func (s *Store) PatchBond(ctx context.Context, scope identity.TenantScope, personID string, patch BondPatch) (Bond, error) {
	if s == nil {
		return Bond{}, fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return Bond{}, err
	}
	personID = normalizePersonID(scope, personID)
	if err := s.ensurePersonBond(ctx, scope.AgentID, personID); err != nil {
		return Bond{}, err
	}
	cur, err := s.GetBond(ctx, scope, personID)
	if err != nil {
		return Bond{}, err
	}

	style, err := ApplyHighField(cur.Style, patch.Style, patch.StyleMode)
	if err != nil {
		return Bond{}, err
	}
	boundaries, err := ApplyHighField(cur.Boundaries, patch.Boundaries, patch.BoundMode)
	if err != nil {
		return Bond{}, err
	}
	basics := MergeMedium(cur.Basics, patch.Basics)
	concerns := MergeMedium(cur.Concerns, patch.Concerns)
	baseline := MergeMedium(cur.Baseline, patch.Baseline)
	strategy := MergeMedium(cur.Strategy, patch.Strategy)
	sources := MergeSourceIDs(cur.SourceEpisodeIDs, patch.SourceEpisodeID)
	now := time.Now().UTC().Format(time.RFC3339)
	conf := cur.Confidence
	if conf < 0.2 {
		conf = 0.2
	}

	err = s.write(ctx, `
MATCH (self:Self {id: $self_id})-[b:BOND]->(p:Person {id: $person_id})
SET b.basics = $basics,
    b.concerns = $concerns,
    b.baseline = $baseline,
    b.strategy = $strategy,
    b.style = $style,
    b.boundaries = $boundaries,
    b.confidence = $confidence,
    b.last_confirmed_at = $now,
    b.source_episode_ids = $sources,
    b.updated_at = $now
`, map[string]any{
		"self_id":    scope.AgentID,
		"person_id":  personID,
		"basics":     basics,
		"concerns":   concerns,
		"baseline":   baseline,
		"strategy":   strategy,
		"style":      style,
		"boundaries": boundaries,
		"confidence": conf,
		"now":        now,
		"sources":    sources,
	})
	if err != nil {
		return Bond{}, err
	}
	return s.GetBond(ctx, scope, personID)
}

func (s *Store) ensurePersonBond(ctx context.Context, selfID, personID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.write(ctx, `
MERGE (self:Self {id: $self_id})
MERGE (p:Person {id: $person_id})
ON CREATE SET p.display_name = $display_name
MERGE (self)-[k:KNOWS]->(p)
ON CREATE SET k.created_at = $now
MERGE (self)-[b:BOND]->(p)
ON CREATE SET
  b.basics = '', b.concerns = '', b.baseline = '', b.strategy = '',
  b.style = '', b.boundaries = '', b.confidence = 0.0,
  b.source_episode_ids = [], b.created_at = $now
`, map[string]any{
		"self_id":      selfID,
		"person_id":    personID,
		"display_name": displayNameFromPersonID(personID),
		"now":          now,
	})
}

// UpsertEpisodePointer writes Episode node + ABOUT edges to Person and/or Self.
func (s *Store) UpsertEpisodePointer(ctx context.Context, scope identity.TenantScope, ep EpisodePointer) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if ep.ID == "" {
		return fmt.Errorf("episode id required")
	}
	created := ep.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}

	aboutSelf := ep.AboutSelf
	var persons []string
	for _, raw := range ep.PersonIDs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if IsSelfSubject(scope, raw) {
			aboutSelf = true
			continue
		}
		persons = append(persons, normalizePersonID(scope, raw))
	}
	if !aboutSelf && len(persons) == 0 {
		persons = []string{scope.PersonID()}
	}
	mode := strings.TrimSpace(ep.ExperienceMode)
	if mode == "" {
		mode = "real_interaction"
	}
	status := strings.TrimSpace(ep.Status)
	if status == "" {
		status = "active"
	}

	return s.write(ctx, `
MERGE (self:Self {id: $self_id})
MERGE (e:Episode {id: $id})
SET e.kind = $kind,
    e.summary = $summary,
    e.doc_uri = $doc_uri,
    e.session_id = $session_id,
    e.created_at = $created_at,
    e.self_id = $self_id,
    e.about_self = $about_self,
    e.experience_mode = $experience_mode,
    e.status = $status,
    e.valid = $valid,
    e.updated_at = $now
WITH self, e
FOREACH (_ IN CASE WHEN $about_self THEN [1] ELSE [] END |
  MERGE (e)-[:ABOUT]->(self)
)
WITH self, e
FOREACH (pid IN $person_ids |
  MERGE (p:Person {id: pid})
  ON CREATE SET p.display_name = pid
  MERGE (e)-[:ABOUT]->(p)
  MERGE (self)-[:KNOWS]->(p)
)
`, map[string]any{
		"self_id":          scope.AgentID,
		"id":               ep.ID,
		"kind":             ep.Kind,
		"summary":          ep.Summary,
		"doc_uri":          ep.DocURI,
		"session_id":       ep.SessionID,
		"created_at":       created.UTC().Format(time.RFC3339),
		"now":              time.Now().UTC().Format(time.RFC3339),
		"about_self":       aboutSelf,
		"experience_mode":  mode,
		"status":           status,
		"valid":            status == "active",
		"person_ids":       persons,
	})
}

// UpsertCalls sets Person -[:CALLS {name}]-> Self.
func (s *Store) UpsertCalls(ctx context.Context, scope identity.TenantScope, personID, name, sourceEpisodeID string) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("call name required")
	}
	personID = normalizePersonID(scope, personID)
	sourceEpisodeID = strings.TrimSpace(sourceEpisodeID)
	now := time.Now().UTC().Format(time.RFC3339)
	return s.write(ctx, `
MERGE (self:Self {id: $self_id})
MERGE (p:Person {id: $person_id})
ON CREATE SET p.display_name = $display_name
MERGE (p)-[c:CALLS]->(self)
SET c.name = $name,
    c.confidence = coalesce(c.confidence, 0.5),
    c.kind = coalesce(c.kind, 'user_said'),
    c.valid_from = coalesce(c.valid_from, $now),
    c.last_confirmed_at = $now,
    c.source_episode_ids = CASE
      WHEN $source_id = '' THEN coalesce(c.source_episode_ids, [])
      WHEN c.source_episode_ids IS NULL THEN [$source_id]
      WHEN NOT $source_id IN c.source_episode_ids THEN c.source_episode_ids + $source_id
      ELSE c.source_episode_ids
    END
`, map[string]any{
		"self_id":      scope.AgentID,
		"person_id":    personID,
		"display_name": displayNameFromPersonID(personID),
		"name":         name,
		"now":          now,
		"source_id":    sourceEpisodeID,
	})
}

// MarkEpisodeStatus sets Episode node valid/status for soft forget.
func (s *Store) MarkEpisodeStatus(ctx context.Context, episodeID, status, reason string) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	episodeID = strings.TrimSpace(episodeID)
	if episodeID == "" {
		return fmt.Errorf("episode id required")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	valid := status == "active" || status == ""
	now := time.Now().UTC().Format(time.RFC3339)
	return s.write(ctx, `
MERGE (e:Episode {id: $id})
SET e.status = $status,
    e.valid = $valid,
    e.invalid_reason = $reason,
    e.updated_at = $now
`, map[string]any{
		"id":     episodeID,
		"status": status,
		"valid":  valid,
		"reason": strings.TrimSpace(reason),
		"now":    now,
	})
}

// TouchLastSeen updates KNOWS/BOND last_seen timestamps without changing norms.
func (s *Store) TouchLastSeen(ctx context.Context, scope identity.TenantScope, personID string) error {
	if s == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	personID = normalizePersonID(scope, personID)
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.ensurePersonBond(ctx, scope.AgentID, personID); err != nil {
		return err
	}
	return s.write(ctx, `
MATCH (self:Self {id: $self_id})-[b:BOND]->(p:Person {id: $person_id})
SET b.last_seen_at = $now
WITH self, p
MATCH (self)-[k:KNOWS]->(p)
SET k.last_seen_at = $now
`, map[string]any{
		"self_id":   scope.AgentID,
		"person_id": personID,
		"now":       now,
	})
}
