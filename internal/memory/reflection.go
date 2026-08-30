package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/identity"
)

type ReflectionScope string

const (
	ReflectionScopeBond               ReflectionScope = "bond"
	ReflectionScopeSelfPattern        ReflectionScope = "self_pattern"
	ReflectionScopeTension            ReflectionScope = "tension"
	ReflectionScopePrincipleCandidate ReflectionScope = "principle_candidate"
)

type ReflectionSource string

const (
	ReflectionSourceDirect    ReflectionSource = "direct_expression"
	ReflectionSourceObserved  ReflectionSource = "behavior_observation"
	ReflectionSourceInferred  ReflectionSource = "model_inference"
	ReflectionSourceGenerated ReflectionSource = "generated_hypothesis"
)

type ReflectionSeedStatus string

const (
	ReflectionSeedOpen       ReflectionSeedStatus = "open"
	ReflectionSeedDeferred   ReflectionSeedStatus = "deferred"
	ReflectionSeedResolved   ReflectionSeedStatus = "resolved"
	ReflectionSeedSuperseded ReflectionSeedStatus = "superseded"
	ReflectionSeedRejected   ReflectionSeedStatus = "rejected"
)

type ReflectionSeed struct {
	ID               string               `json:"id"`
	PersonID         string               `json:"person_id"`
	SessionID        string               `json:"session_id,omitempty"`
	SourceTurnIDs    []string             `json:"source_turn_ids,omitempty"`
	SourceEpisodeIDs []string             `json:"source_episode_ids,omitempty"`
	Scope            ReflectionScope      `json:"scope"`
	Statement        string               `json:"statement"`
	SourceType       ReflectionSource     `json:"source_type"`
	ExperienceModes  []ExperienceMode     `json:"experience_modes,omitempty"`
	Status           ReflectionSeedStatus `json:"status"`
	Generated        bool                 `json:"generated,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
	LastReviewedAt   time.Time            `json:"last_reviewed_at,omitempty"`
	NextReviewAt     time.Time            `json:"next_review_at,omitempty"`
}

type ReflectionSeedWrite struct {
	PersonID         string
	SessionID        string
	SourceTurnIDs    []string
	SourceEpisodeIDs []string
	Scope            ReflectionScope
	Statement        string
	SourceType       ReflectionSource
	ExperienceModes  []ExperienceMode
	Generated        bool
}

type ReflectionEvidenceState string

const (
	ReflectionEvidenceSupport      ReflectionEvidenceState = "support"
	ReflectionEvidenceConflict     ReflectionEvidenceState = "conflict"
	ReflectionEvidenceContext      ReflectionEvidenceState = "context"
	ReflectionEvidenceDuplicate    ReflectionEvidenceState = "duplicate"
	ReflectionEvidenceStale        ReflectionEvidenceState = "stale"
	ReflectionEvidenceInsufficient ReflectionEvidenceState = "insufficient"
	ReflectionEvidenceSuperseded   ReflectionEvidenceState = "superseded"
)

type ReflectionEvidence struct {
	SeedID         string                  `json:"seed_id,omitempty"`
	EpisodeID      string                  `json:"episode_id"`
	State          ReflectionEvidenceState `json:"state"`
	ReasonCode     string                  `json:"reason_code,omitempty"`
	Read           bool                    `json:"read"`
	Generated      bool                    `json:"generated,omitempty"`
	ExperienceMode ExperienceMode          `json:"experience_mode,omitempty"`
	Epistemic      string                  `json:"epistemic,omitempty"`
}

type ReflectionAction string

const (
	ReflectionNoChange       ReflectionAction = "no_change"
	ReflectionDefer          ReflectionAction = "defer"
	ReflectionConfirm        ReflectionAction = "confirm"
	ReflectionRevise         ReflectionAction = "revise"
	ReflectionSupersede      ReflectionAction = "supersede"
	ReflectionOpenTension    ReflectionAction = "open_tension"
	ReflectionResolveTension ReflectionAction = "resolve_tension"
	ReflectionRejectSeed     ReflectionAction = "reject_seed"
)

type ReflectionDecision struct {
	SeedID              string           `json:"seed_id"`
	Action              ReflectionAction `json:"action"`
	CurrentCorrection   bool             `json:"current_correction,omitempty"`
	Kind                ProposalKind     `json:"kind,omitempty"`
	Field               string           `json:"field,omitempty"`
	SuggestedText       string           `json:"suggested_text,omitempty"`
	Mode                string           `json:"mode,omitempty"`
	ReasonSummary       string           `json:"reason_summary,omitempty"`
	ExpectedBondVersion int64            `json:"expected_bond_version,omitempty"`
	ProposalID          string           `json:"proposal_id,omitempty"`
	MutationID          string           `json:"mutation_id,omitempty"`
}

type GeneratedReflectionSeed struct {
	ID        string          `json:"id"`
	Scope     ReflectionScope `json:"scope"`
	Statement string          `json:"statement"`
	Generated bool            `json:"generated"`
}

type ReflectionTrigger string

const (
	ReflectionTriggerReview  ReflectionTrigger = "review"
	ReflectionTriggerDirty   ReflectionTrigger = "dirty"
	ReflectionTriggerManual  ReflectionTrigger = "manual"
	ReflectionTriggerTension ReflectionTrigger = "tension"
)

// ReflectionRun is the public, auditable T3 trajectory. It intentionally omits
// episode bodies and hidden reasoning.
type ReflectionRun struct {
	ID               string                    `json:"id"`
	PersonID         string                    `json:"person_id"`
	SessionID        string                    `json:"session_id,omitempty"`
	Mode             ReflectionMode            `json:"mode"`
	Trigger          ReflectionTrigger         `json:"trigger"`
	SeedIDs          []string                  `json:"seed_ids,omitempty"`
	Queries          []string                  `json:"queries,omitempty"`
	CandidateIDs     []string                  `json:"candidate_ids,omitempty"`
	ReadIDs          []string                  `json:"read_ids,omitempty"`
	Evidence         []ReflectionEvidence      `json:"evidence,omitempty"`
	Decisions        []ReflectionDecision      `json:"decisions,omitempty"`
	MutationIDs      []string                  `json:"mutation_ids,omitempty"`
	GenerativeNote   string                    `json:"generative_note,omitempty"`
	GeneratedSeedIDs []string                  `json:"generated_seed_ids,omitempty"`
	GeneratedSeeds   []GeneratedReflectionSeed `json:"generated_seeds,omitempty"`
	NoChange         bool                      `json:"no_change,omitempty"`
	Notes            string                    `json:"notes,omitempty"`
	Error            string                    `json:"error,omitempty"`
	StartedAt        time.Time                 `json:"started_at"`
	CompletedAt      time.Time                 `json:"completed_at,omitempty"`
	Duration         time.Duration             `json:"duration_ns,omitempty"`
}

type ReviewCheckpoint struct {
	PersonID     string    `json:"person_id"`
	SessionID    string    `json:"session_id"`
	LastTurnID   string    `json:"last_turn_id,omitempty"`
	MessageCount int       `json:"message_count"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

type ReflectionStore struct {
	mu  sync.Mutex
	dir string
}

func NewReflectionStore(dir string) (*ReflectionStore, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "memory", "reflections")
	}
	for _, sub := range []string{"open", "done", "runs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &ReflectionStore{dir: dir}, nil
}

func (s *ReflectionStore) Root() string { return s.dir }

func (s *ReflectionStore) Create(_ context.Context, scope identity.TenantScope, w ReflectionSeedWrite) (ReflectionSeed, error) {
	if err := scope.Validate(); err != nil {
		return ReflectionSeed{}, err
	}
	statement := strings.TrimSpace(w.Statement)
	if statement == "" {
		return ReflectionSeed{}, fmt.Errorf("reflection statement required")
	}
	personID := strings.TrimSpace(w.PersonID)
	if personID == "" {
		personID = scope.PersonID()
	}
	if !strings.Contains(personID, ":") {
		personID = "user:" + personID
	}
	now := time.Now().UTC()
	seed := ReflectionSeed{
		ID: "ref_" + strings.ReplaceAll(uuid.NewString(), "-", ""), PersonID: personID,
		SessionID: strings.TrimSpace(w.SessionID), SourceTurnIDs: uniqueStrings(w.SourceTurnIDs),
		SourceEpisodeIDs: uniqueStrings(w.SourceEpisodeIDs), Scope: normalizeReflectionScope(w.Scope),
		Statement: statement, SourceType: normalizeReflectionSource(w.SourceType),
		ExperienceModes: normalizeExperienceModes(w.ExperienceModes), Status: ReflectionSeedOpen,
		Generated: w.Generated || w.SourceType == ReflectionSourceGenerated, CreatedAt: now, UpdatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeJSONAtomic(filepath.Join(s.dir, "open", seed.ID+".json"), seed); err != nil {
		return ReflectionSeed{}, err
	}
	_ = s.markDirtyLocked(personID, "reflection_seed")
	return seed, nil
}

func (s *ReflectionStore) Get(_ context.Context, id string) (ReflectionSeed, error) {
	id = sanitizeReflectionID(id)
	if id == "" {
		return ReflectionSeed{}, fmt.Errorf("reflection id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range []string{"open", "done"} {
		var seed ReflectionSeed
		if err := readJSON(filepath.Join(s.dir, sub, id+".json"), &seed); err == nil {
			return seed, nil
		}
	}
	return ReflectionSeed{}, fmt.Errorf("reflection not found: %s", id)
}

func (s *ReflectionStore) List(_ context.Context, scope identity.TenantScope, personID string, includeDone bool, limit int) ([]ReflectionSeed, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	personID = strings.TrimSpace(personID)
	if personID != "" && !strings.Contains(personID, ":") {
		personID = "user:" + personID
	}
	subs := []string{"open"}
	if includeDone {
		subs = append(subs, "done")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ReflectionSeed
	for _, sub := range subs {
		entries, err := os.ReadDir(filepath.Join(s.dir, sub))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			var seed ReflectionSeed
			if err := readJSON(filepath.Join(s.dir, sub, entry.Name()), &seed); err != nil {
				continue
			}
			if personID == "" || seed.PersonID == personID {
				out = append(out, seed)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *ReflectionStore) UpdateStatus(_ context.Context, id string, status ReflectionSeedStatus, next time.Time) (ReflectionSeed, error) {
	switch status {
	case ReflectionSeedOpen, ReflectionSeedDeferred, ReflectionSeedResolved, ReflectionSeedSuperseded, ReflectionSeedRejected:
	default:
		return ReflectionSeed{}, fmt.Errorf("invalid reflection status %q", status)
	}
	id = sanitizeReflectionID(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	openPath := filepath.Join(s.dir, "open", id+".json")
	var seed ReflectionSeed
	if err := readJSON(openPath, &seed); err != nil {
		return ReflectionSeed{}, err
	}
	now := time.Now().UTC()
	seed.Status, seed.UpdatedAt, seed.LastReviewedAt, seed.NextReviewAt = status, now, now, next
	target := openPath
	if status == ReflectionSeedResolved || status == ReflectionSeedSuperseded || status == ReflectionSeedRejected {
		target = filepath.Join(s.dir, "done", id+".json")
	}
	if err := writeJSONAtomic(target, seed); err != nil {
		return ReflectionSeed{}, err
	}
	if target != openPath {
		if err := os.Remove(openPath); err != nil {
			return ReflectionSeed{}, err
		}
	}
	return seed, nil
}

func (s *ReflectionStore) AppendRun(run ReflectionRun) (ReflectionRun, error) {
	if run.ID == "" {
		run.ID = "rrun_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.CompletedAt.IsZero() {
		run.CompletedAt = time.Now().UTC()
	}
	if run.Duration <= 0 {
		run.Duration = run.CompletedAt.Sub(run.StartedAt)
	}
	raw, err := json.Marshal(run)
	if err != nil {
		return ReflectionRun{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, "runs", run.CompletedAt.UTC().Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return ReflectionRun{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return ReflectionRun{}, err
	}
	return run, nil
}

func (s *ReflectionStore) ListRuns(limit int) ([]ReflectionRun, error) {
	if limit <= 0 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.dir, "runs"))
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			paths = append(paths, filepath.Join(s.dir, "runs", entry.Name()))
		}
	}
	sort.Strings(paths)
	var out []ReflectionRun
	for i := len(paths) - 1; i >= 0 && len(out) < limit; i-- {
		f, err := os.Open(paths[i])
		if err != nil {
			continue
		}
		var day []ReflectionRun
		scan := bufio.NewScanner(f)
		for scan.Scan() {
			var run ReflectionRun
			if json.Unmarshal(scan.Bytes(), &run) == nil {
				day = append(day, run)
			}
		}
		_ = f.Close()
		for j := len(day) - 1; j >= 0 && len(out) < limit; j-- {
			out = append(out, day[j])
		}
	}
	return out, nil
}

func (s *ReflectionStore) LoadCheckpoint(personID, sessionID string) (ReviewCheckpoint, error) {
	key := checkpointKey(personID, sessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	all := map[string]ReviewCheckpoint{}
	if err := readJSON(filepath.Join(s.dir, "checkpoints.json"), &all); err != nil && !os.IsNotExist(err) {
		return ReviewCheckpoint{}, err
	}
	return all[key], nil
}

func (s *ReflectionStore) SaveCheckpoint(cp ReviewCheckpoint) error {
	if strings.TrimSpace(cp.PersonID) == "" || strings.TrimSpace(cp.SessionID) == "" {
		return fmt.Errorf("checkpoint person and session required")
	}
	if cp.ReviewedAt.IsZero() {
		cp.ReviewedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, "checkpoints.json")
	all := map[string]ReviewCheckpoint{}
	if err := readJSON(path, &all); err != nil && !os.IsNotExist(err) {
		return err
	}
	all[checkpointKey(cp.PersonID, cp.SessionID)] = cp
	return writeJSONAtomic(path, all)
}

func normalizeReflectionScope(v ReflectionScope) ReflectionScope {
	switch v {
	case ReflectionScopeSelfPattern, ReflectionScopeTension, ReflectionScopePrincipleCandidate:
		return v
	default:
		return ReflectionScopeBond
	}
}

func normalizeReflectionSource(v ReflectionSource) ReflectionSource {
	switch v {
	case ReflectionSourceDirect, ReflectionSourceObserved, ReflectionSourceGenerated:
		return v
	default:
		return ReflectionSourceInferred
	}
}

func sanitizeReflectionID(id string) string {
	id = strings.TrimSpace(id)
	var b strings.Builder
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func checkpointKey(personID, sessionID string) string {
	return strings.TrimSpace(personID) + "|" + strings.TrimSpace(sessionID)
}

func uniqueStrings(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func readJSON(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func writeJSONAtomic(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
