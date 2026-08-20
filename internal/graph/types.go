package graph

import (
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/identity"
)

// Bond is the slow-changing relational norm between Self and a Person.
type Bond struct {
	SelfID           string
	PersonID         string
	Basics           string
	Concerns         string
	Baseline         string
	Strategy         string // legacy prose; NOT SoT for T1 compact inject
	Style            string // legacy Interaction prose
	Boundaries       string
	Items            []BondItem // T1 SoT claims; empty → project from legacy prose
	Version          int64      // bumps on successful SoT write
	StrategyCache    string     // derived policy text; not SoT
	StrategyCacheVer int64      // must equal Version to inject
	Confidence       float64
	LastConfirmedAt  time.Time
	SourceEpisodeIDs []string
	CallName         string // from CALLS edge, if any
	RoleAtOrigin     string // from KNOWS edge (weak prior only)
}

// Empty reports whether all narrative fields are blank.
func (b Bond) Empty() bool {
	return strings.TrimSpace(b.Basics) == "" &&
		strings.TrimSpace(b.Concerns) == "" &&
		strings.TrimSpace(b.Baseline) == "" &&
		strings.TrimSpace(b.Strategy) == "" &&
		strings.TrimSpace(b.Style) == "" &&
		strings.TrimSpace(b.Boundaries) == "" &&
		strings.TrimSpace(b.CallName) == ""
}

// FormatRecall renders a T1 compact bond view (no query → no conditional slots).
// Legacy Strategy prose is not treated as SoT and is omitted.
func (b Bond) FormatRecall() string {
	text, _ := FormatCompactRecall(b, "")
	return text
}

// BondPatch is a subject-driven partial update.
type BondPatch struct {
	Basics          string
	Concerns        string
	Baseline        string
	Strategy        string
	Style           string
	Boundaries      string
	StyleMode       string // append (default for high) | replace
	BoundMode       string
	SourceEpisodeID string
}

// EpisodePointer is the L2 index for an L1 episode file.
type EpisodePointer struct {
	ID             string
	Kind           string
	Summary        string
	DocURI         string
	SessionID      string
	CreatedAt      time.Time
	PersonIDs      []string
	AboutSelf      bool // true → (:Episode)-[:ABOUT]->(:Self)
	ExperienceMode string
	Status         string
}

// SelfSubjectMarker is stored in episode person_ids when the subject is Self.
const SelfSubjectMarker = "self"

// IsSelfSubject reports whether an about/person id refers to Self.
func IsSelfSubject(scope identity.TenantScope, raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return false
	}
	switch raw {
	case SelfSubjectMarker, "me", "myself", "i", "agent":
		return true
	}
	agent := strings.ToLower(strings.TrimSpace(scope.AgentID))
	if agent != "" && (raw == agent || raw == "agent:"+agent) {
		return true
	}
	return false
}

// ApplyHighField applies append-only discipline for style/boundaries.
// Returns (newValue, error). replace of non-empty existing is rejected.
func ApplyHighField(existing, incoming, mode string) (string, error) {
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return existing, nil
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "append"
	}
	existing = strings.TrimSpace(existing)
	switch mode {
	case "append":
		if existing == "" {
			return incoming, nil
		}
		if strings.Contains(existing, incoming) {
			return existing, nil
		}
		return existing + "\n" + incoming, nil
	case "replace":
		if existing != "" && existing != incoming {
			return existing, fmt.Errorf("high-threshold field: replace rejected; use append")
		}
		return incoming, nil
	default:
		return existing, fmt.Errorf("unknown mode %q (use append|replace)", mode)
	}
}

// MergeMedium replaces when incoming is non-empty.
func MergeMedium(existing, incoming string) string {
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return existing
	}
	return incoming
}

// MergeSourceIDs appends a source episode id without duplicates.
func MergeSourceIDs(existing []string, id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return existing
	}
	for _, x := range existing {
		if x == id {
			return existing
		}
	}
	return append(append([]string{}, existing...), id)
}

// SummaryFromContent takes the first non-empty line, capped.
func SummaryFromContent(content string, max int) string {
	if max <= 0 {
		max = 160
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	line := content
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		line = strings.TrimSpace(content[:i])
	}
	runes := []rune(line)
	if len(runes) > max {
		return string(runes[:max]) + "…"
	}
	return line
}
