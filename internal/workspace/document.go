// Package workspace stores unfinished thinking documents (file truth).
// Distinct from SelfArtifact: Workspace is "what I'm thinking", not "what I believe".
package workspace

import (
	"fmt"
	"strings"
	"time"
)

// Type is the Workspace document kind.
type Type string

const (
	TypeQuestion Type = "question"
	TypeWriting  Type = "writing"
	TypeResearch Type = "research"
	TypeProject  Type = "project"
)

// Status is the document lifecycle.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusPaused     Status = "paused"
	StatusDone       Status = "done"
	StatusArchived   Status = "archived"
)

// Revision is one append-only history entry.
type Revision struct {
	At      time.Time `json:"at"`
	Summary string    `json:"summary"`
	Actor   string    `json:"actor,omitempty"`
}

// Document is a file-backed Workspace object.
type Document struct {
	ID             string     `json:"id"`
	Type           Type       `json:"type"`
	Status         Status     `json:"status"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Body           string     `json:"body"`
	EpisodeIDs     []string   `json:"episode_ids,omitempty"`
	RelatedSelfIDs []string   `json:"related_self_ids,omitempty"`
	Revisions      []Revision `json:"revisions,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Write is the create payload.
type Write struct {
	Type           Type
	Status         Status
	Title          string
	Summary        string
	Body           string
	EpisodeIDs     []string
	RelatedSelfIDs []string
	Actor          string
	RevisionNote   string
}

// Update is the update payload (partial).
type Update struct {
	Status         Status
	Title          string
	Summary        string
	Body           string
	RelatedSelfIDs []string // if non-nil, replace
	Actor          string
	RevisionNote   string
}

// NormalizeType maps raw type.
func NormalizeType(raw string) Type {
	switch Type(strings.ToLower(strings.TrimSpace(raw))) {
	case TypeWriting:
		return TypeWriting
	case TypeResearch:
		return TypeResearch
	case TypeProject:
		return TypeProject
	default:
		return TypeQuestion
	}
}

// NormalizeStatus maps raw status.
func NormalizeStatus(raw string) Status {
	switch Status(strings.ToLower(strings.TrimSpace(raw))) {
	case StatusInProgress:
		return StatusInProgress
	case StatusPaused:
		return StatusPaused
	case StatusDone:
		return StatusDone
	case StatusArchived:
		return StatusArchived
	default:
		return StatusOpen
	}
}

// DirName returns the subdirectory for a type.
func DirName(t Type) string {
	switch NormalizeType(string(t)) {
	case TypeWriting:
		return "writings"
	case TypeResearch:
		return "research"
	case TypeProject:
		return "projects"
	default:
		return "questions"
	}
}

// IDPrefix returns the id prefix for a type.
func IDPrefix(t Type) string {
	switch NormalizeType(string(t)) {
	case TypeWriting:
		return "ww_"
	case TypeResearch:
		return "wr_"
	case TypeProject:
		return "wp_"
	default:
		return "wq_"
	}
}

func validateWrite(w Write) error {
	if strings.TrimSpace(w.Title) == "" && strings.TrimSpace(w.Body) == "" {
		return fmt.Errorf("title or body required")
	}
	return nil
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	r := []rune(s)
	if max > 0 && len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
