package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"deep-seeing/internal/identity"
)

const (
	memoryIndexName = "MEMORY.md"
	topicsDirName   = "topics"
	maxIndexBytes   = 25 * 1024
	maxIndexLines   = 200
)

var unsafeKey = regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)

// MarkdownStore keeps durable memory as MEMORY.md + topics/*.md (Claude Code style).
type MarkdownStore struct {
	mu  sync.Mutex
	dir string
}

// NewMarkdownStore opens or creates a markdown memory root. If legacy data/ltm.json
// exists beside the parent of dir, records are imported once when the store is empty.
func NewMarkdownStore(dir string) (*MarkdownStore, error) {
	s := &MarkdownStore{dir: dir}
	if err := os.MkdirAll(s.topicsDir(), 0o755); err != nil {
		return nil, err
	}
	if err := s.ensureIndex(); err != nil {
		return nil, err
	}
	if err := s.maybeMigrateLegacyJSON(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *MarkdownStore) Root() string { return s.dir }

func (s *MarkdownStore) topicsDir() string {
	return filepath.Join(s.dir, topicsDirName)
}

func (s *MarkdownStore) indexPath() string {
	return filepath.Join(s.dir, memoryIndexName)
}

func (s *MarkdownStore) topicPath(key string) string {
	return filepath.Join(s.topicsDir(), sanitizeKey(key)+".md")
}

func (s *MarkdownStore) ensureIndex() error {
	p := s.indexPath()
	if _, err := os.Stat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	body := "# Memory Index\n\n<!-- key | category | one-line summary; details live in topics/ -->\n\n"
	return os.WriteFile(p, []byte(body), 0o644)
}

func (s *MarkdownStore) maybeMigrateLegacyJSON() error {
	entries, err := s.listIndexEntriesLocked()
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	legacy := filepath.Join(filepath.Dir(s.dir), "ltm.json")
	data, err := os.ReadFile(legacy)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload struct {
		Records []Record `json:"records"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	scope := identity.LocalCLI()
	for _, r := range payload.Records {
		if err := s.Write(context.Background(), scope, Write{
			Category: r.Category, Key: r.Key, Content: r.Content, Metadata: r.Metadata,
		}); err != nil {
			return err
		}
	}
	_ = os.Rename(legacy, legacy+".migrated")
	return nil
}

func (s *MarkdownStore) Write(_ context.Context, scope identity.TenantScope, w Write) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	key := sanitizeKey(strings.TrimSpace(w.Key))
	content := strings.TrimSpace(w.Content)
	if key == "" || content == "" {
		return fmt.Errorf("memory write: key and content required")
	}
	cat := w.Category
	if cat == "" {
		cat = CategoryUser
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	summary := firstLine(content, 80)
	topic := formatTopicFile(key, cat, content, now)
	if err := os.WriteFile(s.topicPath(key), []byte(topic), 0o644); err != nil {
		return err
	}
	return s.upsertIndexLocked(key, cat, summary)
}

func (s *MarkdownStore) Query(_ context.Context, scope identity.TenantScope, q Query) ([]Record, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 8
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.listIndexEntriesLocked()
	if err != nil {
		return nil, err
	}
	text := strings.ToLower(strings.TrimSpace(q.Text))
	var out []Record
	for _, e := range entries {
		if len(q.Categories) > 0 && !categoryIn(e.Category, q.Categories) {
			continue
		}
		if len(q.Keys) > 0 && !stringIn(e.Key, q.Keys) {
			continue
		}
		rec, err := s.readTopicLocked(e.Key)
		if err != nil {
			continue
		}
		if text != "" {
			hay := strings.ToLower(rec.Key + " " + e.Summary + " " + rec.Content)
			if !strings.Contains(hay, text) && !containsAnyRuneToken(hay, text) {
				continue
			}
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *MarkdownStore) ListRecent(_ context.Context, scope identity.TenantScope, limit int) ([]Record, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 12
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.listIndexEntriesLocked()
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		rec, err := s.readTopicLocked(e.Key)
		if err != nil {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// IndexText returns the capped MEMORY.md body for LLM side-query.
func (s *MarkdownStore) IndexText() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return "", err
	}
	return capIndex(string(data)), nil
}

// ReadKeys loads full topic records for selected keys (order preserved).
func (s *MarkdownStore) ReadKeys(keys []string) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Record
	seen := map[string]bool{}
	for _, key := range keys {
		key = sanitizeKey(strings.TrimSpace(key))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		rec, err := s.readTopicLocked(key)
		if err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

type indexEntry struct {
	Key      string
	Category Category
	Summary  string
}

func (s *MarkdownStore) listIndexEntriesLocked() ([]indexEntry, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return nil, err
	}
	var entries []indexEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		// - `key` (category): summary
		rest := strings.TrimPrefix(line, "- `")
		end := strings.Index(rest, "`")
		if end <= 0 {
			continue
		}
		key := rest[:end]
		rest = strings.TrimSpace(rest[end+1:])
		cat := CategoryUser
		summary := rest
		if strings.HasPrefix(rest, "(") {
			close := strings.Index(rest, ")")
			if close > 1 {
				cat = Category(strings.TrimSpace(rest[1:close]))
				summary = strings.TrimSpace(rest[close+1:])
				summary = strings.TrimPrefix(summary, ":")
				summary = strings.TrimSpace(summary)
			}
		}
		entries = append(entries, indexEntry{Key: key, Category: cat, Summary: summary})
	}
	return entries, nil
}

func (s *MarkdownStore) upsertIndexLocked(key string, cat Category, summary string) error {
	entries, err := s.listIndexEntriesLocked()
	if err != nil {
		return err
	}
	next := []indexEntry{{Key: key, Category: cat, Summary: summary}}
	for _, e := range entries {
		if e.Key == key {
			continue
		}
		next = append(next, e)
	}
	var b strings.Builder
	b.WriteString("# Memory Index\n\n")
	b.WriteString("<!-- newest first; details in topics/<key>.md -->\n\n")
	for _, e := range next {
		fmt.Fprintf(&b, "- `%s` (%s): %s\n", e.Key, e.Category, e.Summary)
	}
	return os.WriteFile(s.indexPath(), []byte(b.String()), 0o644)
}

func (s *MarkdownStore) readTopicLocked(key string) (Record, error) {
	data, err := os.ReadFile(s.topicPath(key))
	if err != nil {
		return Record{}, err
	}
	return parseTopicFile(key, string(data))
}

func formatTopicFile(key string, cat Category, content string, updated time.Time) string {
	return fmt.Sprintf("---\nkey: %s\ncategory: %s\nupdated_at: %s\n---\n\n%s\n",
		key, cat, updated.Format(time.RFC3339), strings.TrimSpace(content))
}

func parseTopicFile(fallbackKey, raw string) (Record, error) {
	rec := Record{Key: fallbackKey, Category: CategoryUser, ID: fallbackKey}
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		parts := strings.SplitN(raw, "---\n", 3)
		if len(parts) >= 3 {
			for _, line := range strings.Split(parts[1], "\n") {
				line = strings.TrimSpace(line)
				if after, ok := strings.CutPrefix(line, "key:"); ok {
					rec.Key = strings.TrimSpace(after)
				}
				if after, ok := strings.CutPrefix(line, "category:"); ok {
					rec.Category = Category(strings.TrimSpace(after))
				}
				if after, ok := strings.CutPrefix(line, "updated_at:"); ok {
					if t, err := time.Parse(time.RFC3339, strings.TrimSpace(after)); err == nil {
						rec.UpdatedAt = t
						rec.CreatedAt = t
					}
				}
			}
			body = parts[2]
		}
	}
	rec.Content = strings.TrimSpace(body)
	if rec.ID == "" {
		rec.ID = rec.Key
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now()
		rec.CreatedAt = rec.UpdatedAt
	}
	return rec, nil
}

func sanitizeKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = unsafeKey.ReplaceAllString(key, "_")
	key = strings.Trim(key, "_")
	if len(key) > 80 {
		key = key[:80]
	}
	return key
}

func firstLine(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}

func capIndex(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxIndexLines {
		lines = lines[:maxIndexLines]
		s = strings.Join(lines, "\n")
	}
	if len(s) > maxIndexBytes {
		s = s[:maxIndexBytes]
	}
	return s
}

func categoryIn(c Category, list []Category) bool {
	for _, x := range list {
		if x == c {
			return true
		}
	}
	return false
}

func stringIn(v string, list []string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func containsAnyRuneToken(hay, query string) bool {
	q := []rune(query)
	if len(q) < 2 {
		return strings.Contains(hay, query)
	}
	for i := 0; i+1 < len(q); i++ {
		tok := string(q[i : i+2])
		if strings.Contains(hay, tok) {
			return true
		}
	}
	return false
}
