package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store is the file-backed Workspace document store.
type Store struct {
	mu  sync.Mutex
	dir string
}

// NewStore creates data/memory/workspace (or custom root) with type subdirs.
func NewStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "memory", "workspace")
	}
	for _, sub := range []string{"questions", "writings", "research", "projects"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Store{dir: dir}, nil
}

// Root returns the store root.
func (s *Store) Root() string { return s.dir }

// Create writes a new workspace document.
func (s *Store) Create(w Write) (Document, error) {
	if err := validateWrite(w); err != nil {
		return Document{}, err
	}
	t := NormalizeType(string(w.Type))
	st := NormalizeStatus(string(w.Status))
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
	d := Document{
		ID: id, Type: t, Status: st, Title: title, Summary: summary,
		Body:           strings.TrimSpace(w.Body),
		EpisodeIDs:     append([]string(nil), w.EpisodeIDs...),
		RelatedSelfIDs: append([]string(nil), w.RelatedSelfIDs...),
		CreatedAt:      now, UpdatedAt: now,
	}
	note := strings.TrimSpace(w.RevisionNote)
	if note == "" {
		note = "created"
	}
	d.Revisions = []Revision{{At: now, Summary: note, Actor: strings.TrimSpace(w.Actor)}}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.WriteFile(s.path(d), []byte(formatDocument(d)), 0o644); err != nil {
		return Document{}, err
	}
	return d, nil
}

// Get loads one document by id (searches type dirs).
func (s *Store) Get(id string) (Document, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Document{}, fmt.Errorf("id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range []Type{TypeQuestion, TypeWriting, TypeResearch, TypeProject} {
		path := filepath.Join(s.dir, DirName(t), id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return parseDocument(id, string(data))
	}
	return Document{}, fmt.Errorf("workspace document not found: %s", id)
}

// Update mutates an existing document and appends a revision.
func (s *Store) Update(id string, u Update) (Document, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Document{}, fmt.Errorf("id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, path, err := s.loadLocked(id)
	if err != nil {
		return Document{}, err
	}
	now := time.Now().UTC()
	changed := false
	if u.Status != "" {
		d.Status = NormalizeStatus(string(u.Status))
		changed = true
	}
	if title := strings.TrimSpace(u.Title); title != "" {
		d.Title = title
		changed = true
	}
	if summary := strings.TrimSpace(u.Summary); summary != "" {
		d.Summary = summary
		changed = true
	}
	if u.Body != "" || strings.TrimSpace(u.RevisionNote) != "" {
		if body := strings.TrimSpace(u.Body); body != "" {
			d.Body = body
			if d.Summary == "" {
				d.Summary = firstLine(body, 160)
			}
			changed = true
		}
	}
	if u.RelatedSelfIDs != nil {
		d.RelatedSelfIDs = append([]string(nil), u.RelatedSelfIDs...)
		changed = true
	}
	if !changed && strings.TrimSpace(u.RevisionNote) == "" {
		return Document{}, fmt.Errorf("nothing to update")
	}
	note := strings.TrimSpace(u.RevisionNote)
	if note == "" {
		note = "updated"
	}
	d.Revisions = append(d.Revisions, Revision{
		At: now, Summary: note, Actor: strings.TrimSpace(u.Actor),
	})
	d.UpdatedAt = now
	if err := os.WriteFile(path, []byte(formatDocument(d)), 0o644); err != nil {
		return Document{}, err
	}
	return d, nil
}

// LinkEpisode appends an episode id (deduped).
func (s *Store) LinkEpisode(id, episodeID string) (Document, error) {
	id = strings.TrimSpace(id)
	episodeID = strings.TrimSpace(episodeID)
	if id == "" || episodeID == "" {
		return Document{}, fmt.Errorf("id and episode_id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, path, err := s.loadLocked(id)
	if err != nil {
		return Document{}, err
	}
	for _, e := range d.EpisodeIDs {
		if e == episodeID {
			return d, nil
		}
	}
	now := time.Now().UTC()
	d.EpisodeIDs = append(d.EpisodeIDs, episodeID)
	d.Revisions = append(d.Revisions, Revision{
		At: now, Summary: "linked episode " + episodeID, Actor: "tool",
	})
	d.UpdatedAt = now
	if err := os.WriteFile(path, []byte(formatDocument(d)), 0o644); err != nil {
		return Document{}, err
	}
	return d, nil
}

// ListFilter selects documents.
type ListFilter struct {
	Type   Type
	Status Status
	Limit  int
}

// Overview is a short card for list views.
type Overview struct {
	ID        string    `json:"id"`
	Type      Type      `json:"type"`
	Status    Status    `json:"status"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	UpdatedAt time.Time `json:"updated_at"`
}

// List returns documents matching filter, newest UpdatedAt first.
func (s *Store) List(filter ListFilter) ([]Document, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	types := []Type{TypeQuestion, TypeWriting, TypeResearch, TypeProject}
	if filter.Type != "" {
		types = []Type{NormalizeType(string(filter.Type))}
	}
	var out []Document
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
			d, err := parseDocument(strings.TrimSuffix(e.Name(), ".md"), string(data))
			if err != nil {
				continue
			}
			if filter.Status != "" && d.Status != NormalizeStatus(string(filter.Status)) {
				continue
			}
			out = append(out, d)
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

func (s *Store) path(d Document) string {
	return filepath.Join(s.dir, DirName(d.Type), d.ID+".md")
}

func (s *Store) loadLocked(id string) (Document, string, error) {
	for _, t := range []Type{TypeQuestion, TypeWriting, TypeResearch, TypeProject} {
		path := filepath.Join(s.dir, DirName(t), id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		d, err := parseDocument(id, string(data))
		if err != nil {
			return Document{}, "", err
		}
		return d, path, nil
	}
	return Document{}, "", fmt.Errorf("workspace document not found: %s", id)
}

// ToOverview converts a document to a list card.
func ToOverview(d Document) Overview {
	return Overview{
		ID: d.ID, Type: d.Type, Status: d.Status, Title: d.Title,
		Summary: d.Summary, UpdatedAt: d.UpdatedAt,
	}
}
