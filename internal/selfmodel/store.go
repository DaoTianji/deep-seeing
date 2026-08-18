package selfmodel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/memory"
)

// Store is the file-backed SelfArtifact store.
type Store struct {
	mu  sync.Mutex
	dir string
}

// NewStore creates data/memory/self (or custom root) with type subdirs.
func NewStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "memory", "self")
	}
	for _, sub := range []string{"patterns", "principles", "tensions", "questions"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Store{dir: dir}, nil
}

// Root returns the store root.
func (s *Store) Root() string { return s.dir }

// Create writes a new artifact document.
func (s *Store) Create(w Write) (Artifact, error) {
	if err := validateWrite(w); err != nil {
		return Artifact{}, err
	}
	t := NormalizeType(string(w.Type))
	st := NormalizeStatus(string(w.Status))
	if st == StatusClaimed && !CanPromoteToPrinciple(w.ExperienceModes) {
		st = MaxStatusForModes(w.ExperienceModes)
	}
	now := time.Now().UTC()
	id := IDPrefix(t) + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	title := strings.TrimSpace(w.Title)
	if title == "" {
		title = firstLine(w.Body, 80)
	}
	summary := strings.TrimSpace(w.Summary)
	if summary == "" {
		summary = firstLine(w.Body, 160)
	}
	a := Artifact{
		ID: id, Type: t, Status: st, Title: title, Summary: summary,
		Body: strings.TrimSpace(w.Body), Confidence: w.Confidence,
		SourceEpisodeIDs: append([]string(nil), w.SourceEpisodeIDs...),
		ExperienceModes:  append([]memory.ExperienceMode(nil), w.ExperienceModes...),
		CreatedAt:        now, UpdatedAt: now,
	}
	note := strings.TrimSpace(w.RevisionNote)
	if note == "" {
		note = "created"
	}
	a.Revisions = []Revision{{At: now, Summary: note, Actor: strings.TrimSpace(w.Actor)}}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.WriteFile(s.path(a), []byte(formatArtifact(a)), 0o644); err != nil {
		return Artifact{}, err
	}
	return a, nil
}

// Get loads one artifact by id (searches type dirs).
func (s *Store) Get(id string) (Artifact, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Artifact{}, fmt.Errorf("id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range []Type{TypePattern, TypePrinciple, TypeTension, TypeQuestion} {
		path := filepath.Join(s.dir, DirName(t), id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return parseArtifact(id, string(data))
	}
	return Artifact{}, fmt.Errorf("artifact not found: %s", id)
}

// ListRecent lists newest artifacts across types (best-effort by mtime via directory order).
func (s *Store) ListRecent(limit int) ([]Artifact, error) {
	if limit <= 0 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Artifact
	for _, t := range []Type{TypePattern, TypePrinciple, TypeTension, TypeQuestion} {
		entries, err := os.ReadDir(filepath.Join(s.dir, DirName(t)))
		if err != nil {
			continue
		}
		for i := len(entries) - 1; i >= 0 && len(out) < limit; i-- {
			e := entries[i]
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
			out = append(out, a)
		}
	}
	return out, nil
}

// DocURI returns relative doc path for graph pointers.
func (s *Store) DocURI(a Artifact) string {
	return filepath.ToSlash(filepath.Join(DirName(a.Type), a.ID+".md"))
}

func (s *Store) path(a Artifact) string {
	return filepath.Join(s.dir, DirName(a.Type), a.ID+".md")
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
