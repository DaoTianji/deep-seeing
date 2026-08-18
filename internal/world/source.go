package world

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

// Source is a persisted provenance record for external content.
type Source struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	FinalURL    string    `json:"final_url,omitempty"`
	Title       string    `json:"title,omitempty"`
	Query       string    `json:"query,omitempty"` // if from search
	ContentType string    `json:"content_type,omitempty"`
	Excerpt     string    `json:"excerpt,omitempty"`
	Body        string    `json:"body"` // may be fenced untrusted text
	FetchedAt   time.Time `json:"fetched_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// SourceStore keeps sources as Markdown under dir.
type SourceStore struct {
	mu  sync.Mutex
	dir string
}

// NewSourceStore creates data/memory/sources (or custom).
func NewSourceStore(dir string) (*SourceStore, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "memory", "sources")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &SourceStore{dir: dir}, nil
}

// Root returns the store directory.
func (s *SourceStore) Root() string { return s.dir }

// Save writes a new source document.
func (s *SourceStore) Save(src Source) (Source, error) {
	if strings.TrimSpace(src.URL) == "" && strings.TrimSpace(src.Body) == "" {
		return Source{}, fmt.Errorf("url or body required")
	}
	now := time.Now().UTC()
	if src.ID == "" {
		src.ID = "src_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	}
	if src.FetchedAt.IsZero() {
		src.FetchedAt = now
	}
	src.CreatedAt = now
	if strings.TrimSpace(src.Excerpt) == "" {
		src.Excerpt = firstRunes(src.Body, 240)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, src.ID+".md")
	if err := os.WriteFile(path, []byte(formatSource(src)), 0o644); err != nil {
		return Source{}, err
	}
	return src, nil
}

// Get loads one source.
func (s *SourceStore) Get(id string) (Source, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Source{}, fmt.Errorf("id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(s.dir, id+".md"))
	if err != nil {
		return Source{}, err
	}
	return parseSource(id, string(data))
}

// ListRecent returns newest sources.
func (s *SourceStore) ListRecent(limit int) ([]Source, error) {
	if limit <= 0 {
		limit = 30
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []Source
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		src, err := parseSource(strings.TrimSuffix(e.Name(), ".md"), string(data))
		if err != nil {
			continue
		}
		out = append(out, src)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].FetchedAt.After(out[j].FetchedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func formatSource(src Source) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", src.ID)
	fmt.Fprintf(&b, "url: %q\n", src.URL)
	if src.FinalURL != "" {
		fmt.Fprintf(&b, "final_url: %q\n", src.FinalURL)
	}
	if src.Title != "" {
		fmt.Fprintf(&b, "title: %q\n", src.Title)
	}
	if src.Query != "" {
		fmt.Fprintf(&b, "query: %q\n", src.Query)
	}
	if src.ContentType != "" {
		fmt.Fprintf(&b, "content_type: %q\n", src.ContentType)
	}
	fmt.Fprintf(&b, "fetched_at: %s\n", src.FetchedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "created_at: %s\n", src.CreatedAt.UTC().Format(time.RFC3339))
	if src.Excerpt != "" {
		fmt.Fprintf(&b, "excerpt: %q\n", src.Excerpt)
	}
	b.WriteString("---\n\n")
	b.WriteString("## Body\n\n")
	b.WriteString(strings.TrimSpace(src.Body))
	b.WriteByte('\n')
	return b.String()
}

func parseSource(fallbackID, raw string) (Source, error) {
	src := Source{ID: fallbackID}
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		parts := strings.SplitN(raw, "---\n", 3)
		if len(parts) >= 3 {
			for _, line := range strings.Split(parts[1], "\n") {
				line = strings.TrimSpace(line)
				if after, ok := strings.CutPrefix(line, "id:"); ok {
					src.ID = strings.TrimSpace(after)
				}
				if after, ok := strings.CutPrefix(line, "url:"); ok {
					src.URL = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "final_url:"); ok {
					src.FinalURL = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "title:"); ok {
					src.Title = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "query:"); ok {
					src.Query = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "content_type:"); ok {
					src.ContentType = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "excerpt:"); ok {
					src.Excerpt = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "fetched_at:"); ok {
					src.FetchedAt, _ = time.Parse(time.RFC3339, strings.TrimSpace(after))
				}
				if after, ok := strings.CutPrefix(line, "created_at:"); ok {
					src.CreatedAt, _ = time.Parse(time.RFC3339, strings.TrimSpace(after))
				}
			}
			body = parts[2]
		}
	}
	src.Body = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(body), "## Body"))
	if src.ID == "" {
		src.ID = fallbackID
	}
	return src, nil
}

func firstRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if max > 0 && len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
