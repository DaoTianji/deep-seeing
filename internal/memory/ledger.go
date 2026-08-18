package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Mutation is one L2 change record — Graph is current state; Ledger is history.
type Mutation struct {
	ID               string         `json:"mutation_id"`
	Timestamp        time.Time      `json:"timestamp"`
	Kind             string         `json:"kind"` // bond_patch | episode_status | principle | other
	SelfID           string         `json:"self_id,omitempty"`
	PersonID         string         `json:"person_id,omitempty"`
	Field            string         `json:"field,omitempty"`
	Before           map[string]any `json:"before,omitempty"`
	After            map[string]any `json:"after,omitempty"`
	SourceEpisodeIDs []string       `json:"source_episode_ids,omitempty"`
	SourceSessionIDs []string       `json:"source_session_ids,omitempty"`
	ProposalID       string         `json:"proposal_id,omitempty"`
	DreamID          string         `json:"dream_id,omitempty"`
	ReviewID         string         `json:"review_id,omitempty"`
	Actor            string         `json:"actor"` // dream | tool | system | human
	ModelVersion     string         `json:"model_version,omitempty"`
	ReasonSummary    string         `json:"reason_summary,omitempty"`
}

// MutationLedger appends JSONL mutation records.
type MutationLedger struct {
	mu  sync.Mutex
	dir string
}

// NewMutationLedger creates data/memory/mutations (or custom dir).
func NewMutationLedger(dir string) (*MutationLedger, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "memory", "mutations")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &MutationLedger{dir: dir}, nil
}

// Root returns ledger directory.
func (l *MutationLedger) Root() string { return l.dir }

func (l *MutationLedger) dayPath(t time.Time) string {
	return filepath.Join(l.dir, t.UTC().Format("2006-01-02")+".jsonl")
}

// Append writes one mutation line.
func (l *MutationLedger) Append(m Mutation) (Mutation, error) {
	if l == nil {
		return Mutation{}, fmt.Errorf("nil ledger")
	}
	if m.ID == "" {
		m.ID = "mut_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now().UTC()
	}
	if m.Actor == "" {
		m.Actor = "system"
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return Mutation{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	path := l.dayPath(m.Timestamp)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Mutation{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return Mutation{}, err
	}
	return m, nil
}

// ListRecent reads up to limit mutations from the latest day files (newest first-ish).
func (l *MutationLedger) ListRecent(limit int) ([]Mutation, error) {
	if l == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, e.Name())
		}
	}
	// chronological filenames → reverse for newest
	var out []Mutation
	for i := len(files) - 1; i >= 0 && len(out) < limit; i-- {
		data, err := os.ReadFile(filepath.Join(l.dir, files[i]))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for j := len(lines) - 1; j >= 0 && len(out) < limit; j-- {
			line := strings.TrimSpace(lines[j])
			if line == "" {
				continue
			}
			var m Mutation
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				continue
			}
			out = append(out, m)
		}
	}
	return out, nil
}
