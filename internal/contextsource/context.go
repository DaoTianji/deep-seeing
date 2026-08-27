package contextsource

import "time"

// Source identifies one semantically distinct context source.
type Source string

const (
	Bond      Source = "bond"
	SceneNorm Source = "scene_norm"
	Workspace Source = "workspace"
	Intent    Source = "intent"
	Proposal  Source = "proposal"
	Episode   Source = "episode"
)

// OrderedSources returns the stable public order used by traces and evaluation.
func OrderedSources() []Source {
	return []Source{Bond, SceneNorm, Workspace, Intent, Proposal, Episode}
}

// Role describes how a source is allowed to participate in an answer.
type Role string

const (
	Baseline   Role = "baseline"
	Guidance   Role = "guidance"
	Task       Role = "task"
	Plan       Role = "plan"
	Hypothesis Role = "hypothesis"
	Evidence   Role = "evidence"
)

// Candidate is a bounded lead. It never contains the complete source body.
type Candidate struct {
	Source       Source         `json:"source"`
	ID           string         `json:"id"`
	Kind         string         `json:"kind,omitempty"`
	Title        string         `json:"title,omitempty"`
	Preview      string         `json:"preview,omitempty"`
	Role         Role           `json:"role"`
	Status       string         `json:"status,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	ReadRequired bool           `json:"read_required"`
}

// RoleFor returns the fixed epistemic role of a source.
func RoleFor(source Source) Role {
	switch source {
	case Bond:
		return Baseline
	case SceneNorm:
		return Guidance
	case Workspace:
		return Task
	case Intent:
		return Plan
	case Proposal:
		return Hypothesis
	case Episode:
		return Evidence
	default:
		return ""
	}
}

func ValidSource(source Source) bool { return RoleFor(source) != "" }
