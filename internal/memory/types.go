package memory

import (
	"context"
	"strings"
	"time"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

// Category aligns with ascentia semantic memory taxonomy (legacy Record path).
type Category string

const (
	CategoryUser     Category = "user"
	CategoryFeedback Category = "feedback"
	CategoryProject  Category = "project"
	CategoryRef      Category = "reference"
	CategoryPerson   Category = "person"
)

// EpisodeKind classifies a worth-remembering event (not a personality dossier).
type EpisodeKind string

const (
	EpisodeEvent            EpisodeKind = "event"
	EpisodePreference       EpisodeKind = "preference"
	EpisodeBoundary         EpisodeKind = "boundary"
	EpisodeStateObservation EpisodeKind = "state_observation"
	EpisodeSelfNote         EpisodeKind = "self_note"
)

// NormalizeEpisodeKind maps model output to an allowed kind.
// trait/personality are demoted to state_observation (no personality dossier writes).
func NormalizeEpisodeKind(raw string) EpisodeKind {
	switch EpisodeKind(strings.ToLower(strings.TrimSpace(raw))) {
	case EpisodePreference:
		return EpisodePreference
	case EpisodeBoundary:
		return EpisodeBoundary
	case EpisodeStateObservation, "trait", "personality":
		return EpisodeStateObservation
	case EpisodeSelfNote, "self", "reflection":
		return EpisodeSelfNote
	default:
		return EpisodeEvent
	}
}

// ResolveEpisodeSubjects decides whether an episode is about Self and/or which people.
// self_note (or about=self/me/myself) hangs on Self; otherwise defaults to the current person.
func ResolveEpisodeSubjects(scope identity.TenantScope, kind EpisodeKind, about string) (aboutSelf bool, personIDs []string) {
	about = strings.TrimSpace(about)
	kind = NormalizeEpisodeKind(string(kind))
	if about == "" {
		if kind == EpisodeSelfNote {
			return true, []string{graph.SelfSubjectMarker}
		}
		return false, []string{scope.PersonID()}
	}
	if graph.IsSelfSubject(scope, about) {
		return true, []string{graph.SelfSubjectMarker}
	}
	if !strings.Contains(about, ":") {
		about = "user:" + about
	}
	return false, []string{about}
}

// IsSelfSubjectID reports a stored person_ids marker for Self.
func IsSelfSubjectID(id string) bool {
	return graph.IsSelfSubject(identity.TenantScope{}, id) || strings.EqualFold(strings.TrimSpace(id), graph.SelfSubjectMarker)
}

// Episode is one worth-remembering event (L1).
type Episode struct {
	ID             string            `json:"id"`
	Kind           EpisodeKind       `json:"kind"`
	Status         EpisodeStatus     `json:"status,omitempty"`
	ExperienceMode ExperienceMode    `json:"experience_mode,omitempty"`
	Content        string            `json:"content"`
	Why            string            `json:"why,omitempty"`
	PersonIDs      []string          `json:"person_ids,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	LegacyKey      string            `json:"legacy_key,omitempty"`
	InvalidReason  string            `json:"invalid_reason,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// ExperienceMode distinguishes how an episode was lived (orthogonal to Kind).
type ExperienceMode string

const (
	ExperienceRealInteraction     ExperienceMode = "real_interaction"
	ExperienceSimulatedRoleplay   ExperienceMode = "simulated_roleplay"
	ExperienceStoryReading        ExperienceMode = "story_reading"
	ExperienceExternalObservation ExperienceMode = "external_observation"
	ExperienceSelfReflection      ExperienceMode = "self_reflection"
)

// NormalizeExperienceMode maps raw mode; empty → real_interaction (legacy default).
func NormalizeExperienceMode(raw string) ExperienceMode {
	switch ExperienceMode(strings.ToLower(strings.TrimSpace(raw))) {
	case ExperienceSimulatedRoleplay:
		return ExperienceSimulatedRoleplay
	case ExperienceStoryReading:
		return ExperienceStoryReading
	case ExperienceExternalObservation:
		return ExperienceExternalObservation
	case ExperienceSelfReflection:
		return ExperienceSelfReflection
	default:
		return ExperienceRealInteraction
	}
}

// EpisodeStatus is soft lifecycle (no physical delete in v1).
type EpisodeStatus string

const (
	EpisodeActive   EpisodeStatus = "active"
	EpisodeArchived EpisodeStatus = "archived"
	EpisodeInvalid  EpisodeStatus = "invalid"
)

// NormalizeEpisodeStatus maps raw status.
func NormalizeEpisodeStatus(raw string) EpisodeStatus {
	switch EpisodeStatus(strings.ToLower(strings.TrimSpace(raw))) {
	case EpisodeArchived:
		return EpisodeArchived
	case EpisodeInvalid:
		return EpisodeInvalid
	default:
		return EpisodeActive
	}
}

// EpisodeWrite is the payload for creating an episode.
type EpisodeWrite struct {
	Kind           EpisodeKind
	ExperienceMode ExperienceMode
	Content        string
	Why            string
	PersonIDs      []string
	SessionID      string
	LegacyKey      string
	Metadata       map[string]string
}

// Record is a durable memory row scoped by tenant (legacy / recall rendering).
type Record struct {
	ID        string            `json:"id"`
	Category  Category          `json:"category"`
	Key       string            `json:"key"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Write describes a new or updated memory payload (legacy Provider).
type Write struct {
	Category Category
	Key      string
	Content  string
	Metadata map[string]string
}

// Query filters memory retrieval.
type Query struct {
	Text       string
	Keys       []string
	Categories []Category
	Limit      int
}

// Provider persists and queries tenant-scoped memories (LTM).
type Provider interface {
	Write(ctx context.Context, scope identity.TenantScope, w Write) error
	Query(ctx context.Context, scope identity.TenantScope, q Query) ([]Record, error)
	ListRecent(ctx context.Context, scope identity.TenantScope, limit int) ([]Record, error)
}

// EpisodeIndex is the side-query surface over episodes.
type EpisodeIndex interface {
	IndexText(ctx context.Context, scope identity.TenantScope) (string, error)
	ReadIDs(ctx context.Context, scope identity.TenantScope, ids []string) ([]Episode, error)
	ListRecentEpisodes(ctx context.Context, scope identity.TenantScope, limit int) ([]Episode, error)
}

// SideQuerySelector selects relevant memories before the main completion.
type SideQuerySelector interface {
	SelectForTurn(ctx context.Context, scope identity.TenantScope, query string, limit int) ([]Record, error)
}

// PostTurnExtractor runs after a successful user-visible turn.
type PostTurnExtractor interface {
	AfterTurn(ctx context.Context, scope identity.TenantScope, sessionID string, turnUser, turnAssistant string) error
}

// NoopPostTurn is a no-op extractor (default: agent decides what to remember via tools).
type NoopPostTurn struct{}

func (NoopPostTurn) AfterTurn(context.Context, identity.TenantScope, string, string, string) error {
	return nil
}

// EpisodeToRecord maps an episode into the recall Record shape.
func EpisodeToRecord(ep Episode) Record {
	meta := map[string]string{"kind": string(ep.Kind)}
	if ep.Status != "" {
		meta["status"] = string(ep.Status)
	}
	if ep.Why != "" {
		meta["why"] = ep.Why
	}
	if len(ep.PersonIDs) > 0 {
		meta["about"] = ep.PersonIDs[0]
	}
	for k, v := range ep.Metadata {
		meta[k] = v
	}
	key := ep.ID
	if ep.LegacyKey != "" {
		key = ep.LegacyKey
	}
	return Record{
		ID:        ep.ID,
		Category:  CategoryUser,
		Key:       key,
		Content:   ep.Content,
		Metadata:  meta,
		CreatedAt: ep.CreatedAt,
		UpdatedAt: ep.UpdatedAt,
	}
}

// IsActiveEpisode reports whether an episode should appear in default recall.
func IsActiveEpisode(ep Episode) bool {
	st := ep.Status
	if st == "" {
		st = EpisodeActive
	}
	return st == EpisodeActive
}
