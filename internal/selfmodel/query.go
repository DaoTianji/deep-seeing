package selfmodel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ListFilter selects SelfArtifacts for inspect.
type ListFilter struct {
	Type   Type   // empty = all
	Status Status // empty = all
	Limit  int
}

// List returns artifacts matching filter, newest UpdatedAt first.
func (s *Store) List(filter ListFilter) ([]Artifact, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	types := []Type{TypePattern, TypePrinciple, TypeTension, TypeQuestion}
	if filter.Type != "" {
		types = []Type{NormalizeType(string(filter.Type))}
	}
	var out []Artifact
	for _, t := range types {
		entries, err := os.ReadDir(filepath.Join(s.dir, DirName(t)))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(s.dir, DirName(t), e.Name()))
			if err != nil {
				continue
			}
			a, err := parseArtifact(strings.TrimSuffix(e.Name(), ".md"), string(data))
			if err != nil {
				continue
			}
			if filter.Status != "" && a.Status != NormalizeStatus(string(filter.Status)) {
				continue
			}
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ListTensions lists open (non-deprecated) tensions.
func (s *Store) ListTensions(limit int) ([]Artifact, error) {
	all, err := s.List(ListFilter{Type: TypeTension, Limit: limit * 2})
	if err != nil {
		return nil, err
	}
	var out []Artifact
	for _, a := range all {
		if a.Status == StatusDeprecated {
			continue
		}
		out = append(out, a)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// BeliefTrace is the evidence view for one SelfArtifact.
type BeliefTrace struct {
	Artifact         Artifact `json:"artifact"`
	DocURI           string   `json:"doc_uri"`
	SupportedBy      []string `json:"supported_by,omitempty"`
	ChallengedBy     []string `json:"challenged_by,omitempty"`
	TensionWith      []string `json:"tension_with,omitempty"`
	SourceEpisodeIDs []string `json:"source_episode_ids,omitempty"`
	ExperienceModes  []string `json:"experience_modes,omitempty"`
	MaxStatusHint    Status   `json:"max_status_hint"`
	CanBePrinciple   bool     `json:"can_be_principle"`
}

// TraceBelief loads an artifact and assembles evidence (file + optional graph IDs).
func (s *Store) TraceBelief(id string, graphEv *GraphEvidence) (BeliefTrace, error) {
	a, err := s.Get(id)
	if err != nil {
		return BeliefTrace{}, err
	}
	modes := make([]string, 0, len(a.ExperienceModes))
	for _, m := range a.ExperienceModes {
		modes = append(modes, string(m))
	}
	tr := BeliefTrace{
		Artifact: a, DocURI: s.DocURI(a),
		SourceEpisodeIDs: append([]string(nil), a.SourceEpisodeIDs...),
		ExperienceModes:  modes,
		MaxStatusHint:    MaxStatusForModes(a.ExperienceModes),
		CanBePrinciple:   CanPromoteToPrinciple(a.ExperienceModes),
		SupportedBy:      append([]string(nil), a.SourceEpisodeIDs...),
	}
	if graphEv != nil {
		if len(graphEv.SupportedBy) > 0 {
			tr.SupportedBy = append([]string(nil), graphEv.SupportedBy...)
		}
		tr.ChallengedBy = append([]string(nil), graphEv.ChallengedBy...)
		tr.TensionWith = append([]string(nil), graphEv.TensionWith...)
	}
	return tr, nil
}

// GraphEvidence is optional Neo4j-side evidence for TraceBelief.
type GraphEvidence struct {
	SupportedBy  []string
	ChallengedBy []string
	TensionWith  []string
}

// InspectSummary is a compact Self overview.
type InspectSummary struct {
	Counts   map[string]int     `json:"counts"`
	Recent   []ArtifactOverview `json:"recent"`
	Tensions []ArtifactOverview `json:"tensions,omitempty"`
}

// ArtifactOverview is a short card for inspect lists.
type ArtifactOverview struct {
	ID         string    `json:"id"`
	Type       Type      `json:"type"`
	Status     Status    `json:"status"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Confidence float64   `json:"confidence"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Inspect builds an overview of SelfArtifacts.
func (s *Store) Inspect(limit int) (InspectSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	all, err := s.List(ListFilter{Limit: 200})
	if err != nil {
		return InspectSummary{}, err
	}
	counts := map[string]int{
		"pattern": 0, "principle": 0, "tension": 0, "question": 0,
	}
	var recent []ArtifactOverview
	var tensions []ArtifactOverview
	for _, a := range all {
		counts[string(a.Type)]++
		ov := toOverview(a)
		if len(recent) < limit {
			recent = append(recent, ov)
		}
		if a.Type == TypeTension && a.Status != StatusDeprecated && len(tensions) < limit {
			tensions = append(tensions, ov)
		}
	}
	return InspectSummary{Counts: counts, Recent: recent, Tensions: tensions}, nil
}

func toOverview(a Artifact) ArtifactOverview {
	return ArtifactOverview{
		ID: a.ID, Type: a.Type, Status: a.Status, Title: a.Title,
		Summary: a.Summary, Confidence: a.Confidence, UpdatedAt: a.UpdatedAt,
	}
}

// MapProposalKindToType maps proposal kind → artifact type.
func MapProposalKindToType(kind string) (Type, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "self_pattern", "pattern":
		return TypePattern, nil
	case "principle":
		return TypePrinciple, nil
	case "tension":
		return TypeTension, nil
	case "question":
		return TypeQuestion, nil
	default:
		return "", fmt.Errorf("unsupported self proposal kind %q", kind)
	}
}
