// Package selfmodel stores SelfArtifact documents (file truth) and graph pointers.
package selfmodel

import (
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/memory"
)

// Type is the SelfArtifact kind.
type Type string

const (
	TypePattern   Type = "pattern"
	TypePrinciple Type = "principle"
	TypeTension   Type = "tension"
	TypeQuestion  Type = "question"
)

// Status is the claim lifecycle.
type Status string

const (
	StatusObservation Status = "observation"
	StatusTentative   Status = "tentative"
	StatusClaimed     Status = "claimed"
	StatusDeprecated  Status = "deprecated"
)

// Revision is one append-only history entry.
type Revision struct {
	At      time.Time `json:"at"`
	Summary string    `json:"summary"`
	Actor   string    `json:"actor,omitempty"`
}

// Artifact is the file-backed Self cognitive object.
type Artifact struct {
	ID               string                  `json:"id"`
	Type             Type                    `json:"type"`
	Status           Status                  `json:"status"`
	Title            string                  `json:"title"`
	Summary          string                  `json:"summary"`
	Body             string                  `json:"body"`
	Confidence       float64                 `json:"confidence"`
	SourceEpisodeIDs []string                `json:"source_episode_ids,omitempty"`
	ExperienceModes  []memory.ExperienceMode `json:"experience_modes,omitempty"`
	Revisions        []Revision              `json:"revisions,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

// Write is the create/update payload.
type Write struct {
	Type             Type
	Status           Status
	Title            string
	Summary          string
	Body             string
	Confidence       float64
	SourceEpisodeIDs []string
	ExperienceModes  []memory.ExperienceMode
	Actor            string
	RevisionNote     string
}

// NormalizeType maps raw type.
func NormalizeType(raw string) Type {
	switch Type(strings.ToLower(strings.TrimSpace(raw))) {
	case TypePrinciple:
		return TypePrinciple
	case TypeTension:
		return TypeTension
	case TypeQuestion:
		return TypeQuestion
	default:
		return TypePattern
	}
}

// NormalizeStatus maps raw status.
func NormalizeStatus(raw string) Status {
	switch Status(strings.ToLower(strings.TrimSpace(raw))) {
	case StatusTentative:
		return StatusTentative
	case StatusClaimed:
		return StatusClaimed
	case StatusDeprecated:
		return StatusDeprecated
	default:
		return StatusObservation
	}
}

// DirName returns the subdirectory for a type.
func DirName(t Type) string {
	switch NormalizeType(string(t)) {
	case TypePrinciple:
		return "principles"
	case TypeTension:
		return "tensions"
	case TypeQuestion:
		return "questions"
	default:
		return "patterns"
	}
}

// IDPrefix returns the id prefix for a type.
func IDPrefix(t Type) string {
	switch NormalizeType(string(t)) {
	case TypePrinciple:
		return "pr_"
	case TypeTension:
		return "tn_"
	case TypeQuestion:
		return "oq_"
	default:
		return "sp_"
	}
}

func validateWrite(w Write) error {
	if strings.TrimSpace(w.Title) == "" && strings.TrimSpace(w.Body) == "" {
		return fmt.Errorf("title or body required")
	}
	return nil
}
