package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/identity"
)

// ProposalStatus is the lifecycle of a bond update proposal.
type ProposalStatus string

const (
	ProposalOpen     ProposalStatus = "open"
	ProposalAccepted ProposalStatus = "accepted"
	ProposalRejected ProposalStatus = "rejected"
	ProposalDeferred ProposalStatus = "deferred"
)

// Hypothesis is the dual-hypothesis label for a deviation.
type Hypothesis string

const (
	HypothesisNone Hypothesis = "none"
	HypothesisH1   Hypothesis = "H1" // my model may be wrong
	HypothesisH2   Hypothesis = "H2" // their state may be off-baseline
)

// BondProposal is a queued change — never auto-applied without Dream.
type BondProposal struct {
	ID              string           `json:"id"`
	Kind            ProposalKind     `json:"kind,omitempty"` // bond|self_pattern|principle|tension；缺省 bond
	PersonID        string           `json:"person_id"`
	SessionID       string           `json:"session_id,omitempty"`
	Status          ProposalStatus   `json:"status"`
	Hypothesis      Hypothesis       `json:"hypothesis"`
	Field           string           `json:"field"` // bond field 或 self 标题提示
	SuggestedText   string           `json:"suggested_text"`
	Mode            string           `json:"mode,omitempty"` // append|replace
	Rationale       string           `json:"rationale,omitempty"`
	Source          string           `json:"source,omitempty"` // session_review | tool
	ExperienceModes []ExperienceMode `json:"experience_modes,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// ProposalKind distinguishes slow-change domains sharing one queue.
type ProposalKind string

const (
	ProposalKindBond        ProposalKind = "bond"
	ProposalKindSelfPattern ProposalKind = "self_pattern"
	ProposalKindPrinciple   ProposalKind = "principle"
	ProposalKindTension     ProposalKind = "tension"
)

// NormalizeProposalKind maps raw kind; empty → bond.
func NormalizeProposalKind(raw string) ProposalKind {
	switch ProposalKind(strings.ToLower(strings.TrimSpace(raw))) {
	case ProposalKindSelfPattern:
		return ProposalKindSelfPattern
	case ProposalKindPrinciple:
		return ProposalKindPrinciple
	case ProposalKindTension:
		return ProposalKindTension
	default:
		return ProposalKindBond
	}
}

// ProposalWrite is the payload to enqueue a proposal.
type ProposalWrite struct {
	Kind            ProposalKind
	PersonID        string
	SessionID       string
	Hypothesis      Hypothesis
	Field           string
	SuggestedText   string
	Mode            string
	Rationale       string
	Source          string
	ExperienceModes []ExperienceMode
}

// ProposalStore keeps bond proposals as Markdown under dir.
type ProposalStore struct {
	mu  sync.Mutex
	dir string
}

var unsafeProposalToken = regexp.MustCompile(`[^a-zA-Z0-9_\-:]+`)

// NewProposalStore opens or creates a proposal queue root.
func NewProposalStore(dir string) (*ProposalStore, error) {
	s := &ProposalStore{dir: dir}
	for _, sub := range []string{"open", "done"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Root returns the proposal directory.
func (s *ProposalStore) Root() string { return s.dir }

// Enqueue writes a new open proposal.
func (s *ProposalStore) Enqueue(_ context.Context, scope identity.TenantScope, w ProposalWrite) (BondProposal, error) {
	if err := scope.Validate(); err != nil {
		return BondProposal{}, err
	}
	kind := NormalizeProposalKind(string(w.Kind))
	var field string
	if kind == ProposalKindBond {
		field = normalizeProposalField(w.Field)
	} else {
		field = strings.TrimSpace(w.Field)
	}
	text := strings.TrimSpace(w.SuggestedText)
	if text == "" {
		return BondProposal{}, fmt.Errorf("proposal text required")
	}
	if kind == ProposalKindBond && field == "" {
		return BondProposal{}, fmt.Errorf("proposal field and text required")
	}
	personID := strings.TrimSpace(w.PersonID)
	if personID == "" {
		personID = scope.PersonID()
	}
	if !strings.Contains(personID, ":") {
		personID = "user:" + personID
	}
	now := time.Now().UTC()
	p := BondProposal{
		ID:   "prop_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Kind: kind, PersonID: personID, SessionID: strings.TrimSpace(w.SessionID),
		Status: ProposalOpen, Hypothesis: normalizeHypothesis(string(w.Hypothesis)),
		Field: field, SuggestedText: text, Mode: normalizeProposalMode(field, w.Mode),
		Rationale: strings.TrimSpace(w.Rationale), Source: strings.TrimSpace(w.Source),
		ExperienceModes: normalizeExperienceModes(w.ExperienceModes),
		CreatedAt:       now, UpdatedAt: now,
	}
	if p.Source == "" {
		p.Source = "tool"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, "open", sanitizeProposalID(p.ID)+".md")
	if err := os.WriteFile(path, []byte(formatProposalFile(p)), 0o644); err != nil {
		return BondProposal{}, err
	}
	return p, nil
}

// ListOpen returns open proposals, optionally filtered by person.
func (s *ProposalStore) ListOpen(_ context.Context, scope identity.TenantScope, personID string, limit int) ([]BondProposal, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	personID = strings.TrimSpace(personID)
	if personID != "" && !strings.Contains(personID, ":") {
		personID = "user:" + personID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.dir, "open"))
	if err != nil {
		return nil, err
	}
	var out []BondProposal
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, "open", e.Name()))
		if err != nil {
			continue
		}
		p, err := parseProposalFile(string(data))
		if err != nil || p.Status != ProposalOpen {
			continue
		}
		if personID != "" && p.PersonID != personID {
			continue
		}
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Get loads one open proposal by id.
func (s *ProposalStore) Get(_ context.Context, id string) (BondProposal, error) {
	id = sanitizeProposalID(strings.TrimSpace(id))
	if id == "" {
		return BondProposal{}, fmt.Errorf("proposal id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(s.dir, "open", id+".md"))
	if err != nil {
		return BondProposal{}, err
	}
	return parseProposalFile(string(data))
}

// Resolve moves an open proposal to done with a terminal status.
func (s *ProposalStore) Resolve(_ context.Context, id string, status ProposalStatus) (BondProposal, error) {
	switch status {
	case ProposalAccepted, ProposalRejected, ProposalDeferred:
	default:
		return BondProposal{}, fmt.Errorf("invalid resolve status %q", status)
	}
	id = sanitizeProposalID(strings.TrimSpace(id))
	s.mu.Lock()
	defer s.mu.Unlock()
	openPath := filepath.Join(s.dir, "open", id+".md")
	data, err := os.ReadFile(openPath)
	if err != nil {
		return BondProposal{}, err
	}
	p, err := parseProposalFile(string(data))
	if err != nil {
		return BondProposal{}, err
	}
	p.Status = status
	p.UpdatedAt = time.Now().UTC()
	donePath := filepath.Join(s.dir, "done", id+".md")
	if err := os.WriteFile(donePath, []byte(formatProposalFile(p)), 0o644); err != nil {
		return BondProposal{}, err
	}
	if err := os.Remove(openPath); err != nil {
		return BondProposal{}, err
	}
	return p, nil
}

// CountOpen returns how many open proposals exist (optionally for person).
func (s *ProposalStore) CountOpen(ctx context.Context, scope identity.TenantScope, personID string) (int, error) {
	list, err := s.ListOpen(ctx, scope, personID, 1000)
	if err != nil {
		return 0, err
	}
	return len(list), nil
}

// FormatOpenRecall renders open proposals for SideQuery.
func FormatOpenRecall(props []BondProposal) string {
	if len(props) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("待 Dream 采纳的 Bond 提案（仅参考，本阶段不自动改写常模）：\n")
	for _, p := range props {
		b.WriteString(fmt.Sprintf("- [%s] %s.%s (%s): %s", p.ID, p.PersonID, p.Field, p.Hypothesis, p.SuggestedText))
		if p.Rationale != "" {
			b.WriteString(" — ")
			b.WriteString(p.Rationale)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func normalizeProposalField(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "basics", "concerns", "baseline", "strategy", "style", "boundaries":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeProposalMode(field, mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if field == "style" || field == "boundaries" {
		if mode == "replace" {
			return "append" // Phase 3 proposals never ask Dream to hard-replace high fields from one session
		}
		return "append"
	}
	if mode == "" {
		return "replace"
	}
	return mode
}

func normalizeHypothesis(raw string) Hypothesis {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "H1":
		return HypothesisH1
	case "H2":
		return HypothesisH2
	default:
		return HypothesisNone
	}
}

func sanitizeProposalID(id string) string {
	id = strings.TrimSpace(id)
	id = unsafeProposalToken.ReplaceAllString(id, "_")
	return id
}

func formatProposalFile(p BondProposal) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %s\n", p.ID))
	kind := p.Kind
	if kind == "" {
		kind = ProposalKindBond
	}
	b.WriteString(fmt.Sprintf("kind: %s\n", kind))
	b.WriteString(fmt.Sprintf("person_id: %s\n", p.PersonID))
	if p.SessionID != "" {
		b.WriteString(fmt.Sprintf("session_id: %s\n", p.SessionID))
	}
	b.WriteString(fmt.Sprintf("status: %s\n", p.Status))
	b.WriteString(fmt.Sprintf("hypothesis: %s\n", p.Hypothesis))
	b.WriteString(fmt.Sprintf("field: %s\n", p.Field))
	b.WriteString(fmt.Sprintf("mode: %s\n", p.Mode))
	b.WriteString(fmt.Sprintf("source: %s\n", p.Source))
	if len(p.ExperienceModes) > 0 {
		parts := make([]string, 0, len(p.ExperienceModes))
		for _, m := range p.ExperienceModes {
			parts = append(parts, string(m))
		}
		b.WriteString(fmt.Sprintf("experience_modes: %s\n", strings.Join(parts, ",")))
	}
	b.WriteString(fmt.Sprintf("created_at: %s\n", p.CreatedAt.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("updated_at: %s\n", p.UpdatedAt.UTC().Format(time.RFC3339)))
	b.WriteString("---\n\n")
	if p.Rationale != "" {
		b.WriteString("## 理由\n")
		b.WriteString(p.Rationale)
		b.WriteString("\n\n")
	}
	b.WriteString("## 建议文本\n")
	b.WriteString(p.SuggestedText)
	b.WriteByte('\n')
	return b.String()
}

func parseProposalFile(raw string) (BondProposal, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "---") {
		return BondProposal{}, fmt.Errorf("missing front matter")
	}
	rest := strings.TrimPrefix(raw, "---")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return BondProposal{}, fmt.Errorf("bad front matter")
	}
	meta := rest[:idx]
	body := strings.TrimSpace(rest[idx+4:])
	p := BondProposal{Status: ProposalOpen, Hypothesis: HypothesisNone, Mode: "append", Kind: ProposalKindBond}
	for _, line := range strings.Split(meta, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		k, v, _ := strings.Cut(line, ":")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "id":
			p.ID = v
		case "kind":
			p.Kind = NormalizeProposalKind(v)
		case "person_id":
			p.PersonID = v
		case "session_id":
			p.SessionID = v
		case "status":
			p.Status = ProposalStatus(v)
		case "hypothesis":
			p.Hypothesis = normalizeHypothesis(v)
		case "field":
			if NormalizeProposalKind(string(p.Kind)) == ProposalKindBond {
				p.Field = normalizeProposalField(v)
			} else {
				p.Field = strings.TrimSpace(v)
			}
		case "mode":
			p.Mode = v
		case "source":
			p.Source = v
		case "experience_modes":
			p.ExperienceModes = parseExperienceModesCSV(v)
		case "created_at":
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				p.CreatedAt = t
			}
		case "updated_at":
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				p.UpdatedAt = t
			}
		}
	}
	// body: optional ## 理由 / ## 建议文本
	suggested := body
	rationale := ""
	if i := strings.Index(body, "## 建议文本"); i >= 0 {
		suggested = strings.TrimSpace(body[i+len("## 建议文本"):])
		pre := strings.TrimSpace(body[:i])
		if strings.HasPrefix(pre, "## 理由") {
			rationale = strings.TrimSpace(strings.TrimPrefix(pre, "## 理由"))
		}
	}
	p.SuggestedText = suggested
	if rationale != "" {
		p.Rationale = rationale
	}
	if p.ID == "" || p.SuggestedText == "" {
		return BondProposal{}, fmt.Errorf("incomplete proposal")
	}
	if NormalizeProposalKind(string(p.Kind)) == ProposalKindBond && p.Field == "" {
		return BondProposal{}, fmt.Errorf("incomplete proposal")
	}
	return p, nil
}

func normalizeExperienceModes(modes []ExperienceMode) []ExperienceMode {
	if len(modes) == 0 {
		return nil
	}
	out := make([]ExperienceMode, 0, len(modes))
	seen := map[ExperienceMode]bool{}
	for _, m := range modes {
		nm := NormalizeExperienceMode(string(m))
		if seen[nm] {
			continue
		}
		seen[nm] = true
		out = append(out, nm)
	}
	return out
}

func parseExperienceModesCSV(raw string) []ExperienceMode {
	parts := strings.Split(raw, ",")
	var out []ExperienceMode
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, NormalizeExperienceMode(p))
	}
	return normalizeExperienceModes(out)
}
