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

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

const (
	episodeIndexName     = "index.md"
	episodeByIDDirName   = "by_id"
	episodeMigrateMark   = ".migrated_from_topics"
	maxEpisodeIndexBytes = 25 * 1024
	maxEpisodeIndexLines = 200
)

var unsafeEpisodeToken = regexp.MustCompile(`[^a-zA-Z0-9_\-:]+`)

// EpisodeStore keeps L1 episodes as index.md + by_id/*.md.
type EpisodeStore struct {
	mu  sync.Mutex
	dir string
}

// NewEpisodeStore opens or creates an episode root under dir.
// If empty and legacy topics exist beside parent memory root, migrates once.
func NewEpisodeStore(dir string) (*EpisodeStore, error) {
	s := &EpisodeStore{dir: dir}
	if err := os.MkdirAll(s.byIDDir(), 0o755); err != nil {
		return nil, err
	}
	if err := s.ensureIndex(); err != nil {
		return nil, err
	}
	if err := s.maybeMigrateTopics(); err != nil {
		return nil, err
	}
	return s, nil
}

// Root returns the episode store directory.
func (s *EpisodeStore) Root() string { return s.dir }

func (s *EpisodeStore) byIDDir() string {
	return filepath.Join(s.dir, episodeByIDDirName)
}

func (s *EpisodeStore) indexPath() string {
	return filepath.Join(s.dir, episodeIndexName)
}

func (s *EpisodeStore) episodePath(id string) string {
	return filepath.Join(s.byIDDir(), sanitizeEpisodeID(id)+".md")
}

func (s *EpisodeStore) ensureIndex() error {
	p := s.indexPath()
	if _, err := os.Stat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	body := "# Episode Index\n\n<!-- newest first; details in by_id/<id>.md -->\n\n"
	return os.WriteFile(p, []byte(body), 0o644)
}

func (s *EpisodeStore) maybeMigrateTopics() error {
	mark := filepath.Join(s.dir, episodeMigrateMark)
	if _, err := os.Stat(mark); err == nil {
		return nil
	}
	entries, err := os.ReadDir(s.byIDDir())
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			return nil // already has episodes
		}
	}
	// parent of episodes/ is usually data/memory
	memoryRoot := filepath.Dir(s.dir)
	topicsDir := filepath.Join(memoryRoot, "topics")
	topicFiles, err := filepath.Glob(filepath.Join(topicsDir, "*.md"))
	if err != nil || len(topicFiles) == 0 {
		_ = os.WriteFile(mark, []byte("no topics\n"), 0o644)
		return nil
	}
	scope := identity.LocalCLI()
	ctx := context.Background()
	for _, tf := range topicFiles {
		data, err := os.ReadFile(tf)
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(tf), ".md")
		rec, err := parseTopicFile(base, string(data))
		if err != nil || strings.TrimSpace(rec.Content) == "" {
			continue
		}
		_, err = s.WriteEpisode(ctx, scope, EpisodeWrite{
			Kind:      EpisodeEvent,
			Content:   rec.Content,
			PersonIDs: []string{scope.PersonID()},
			LegacyKey: rec.Key,
		})
		if err != nil {
			return err
		}
	}
	return os.WriteFile(mark, []byte("ok\n"), 0o644)
}

// WriteEpisode creates a new episode file and prepends the index.
func (s *EpisodeStore) WriteEpisode(ctx context.Context, scope identity.TenantScope, w EpisodeWrite) (Episode, error) {
	_ = ctx
	if err := scope.Validate(); err != nil {
		return Episode{}, err
	}
	content := strings.TrimSpace(w.Content)
	if content == "" {
		return Episode{}, fmt.Errorf("episode content required")
	}
	kind := w.Kind
	if kind == "" {
		kind = EpisodeEvent
	}
	kind = NormalizeEpisodeKind(string(kind))
	mode := NormalizeExperienceMode(string(w.ExperienceMode))
	persons := append([]string(nil), w.PersonIDs...)
	if len(persons) == 0 {
		if kind == EpisodeSelfNote {
			persons = []string{graph.SelfSubjectMarker}
		} else {
			persons = []string{scope.PersonID()}
		}
	}
	now := time.Now()
	id := "ep_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ep := Episode{
		ID:             id,
		Kind:           kind,
		Status:         EpisodeActive,
		ExperienceMode: mode,
		Content:        content,
		Why:            strings.TrimSpace(w.Why),
		PersonIDs:      persons,
		SessionID:      strings.TrimSpace(w.SessionID),
		LegacyKey:      strings.TrimSpace(w.LegacyKey),
		Metadata:       w.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.WriteFile(s.episodePath(id), []byte(formatEpisodeFile(ep)), 0o644); err != nil {
		return Episode{}, err
	}
	if err := s.prependIndexLocked(ep); err != nil {
		return Episode{}, err
	}
	return ep, nil
}

// Get loads one episode by id.
func (s *EpisodeStore) Get(_ context.Context, id string) (Episode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readEpisodeLocked(id)
}

// SetStatus archives or invalidates an episode without physical delete.
func (s *EpisodeStore) SetStatus(_ context.Context, id string, status EpisodeStatus, reason string) (Episode, error) {
	status = NormalizeEpisodeStatus(string(status))
	if status == EpisodeActive {
		return Episode{}, fmt.Errorf("use WriteEpisode path to create active episodes")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, err := s.readEpisodeLocked(id)
	if err != nil {
		return Episode{}, err
	}
	ep.Status = status
	ep.UpdatedAt = time.Now().UTC()
	if status == EpisodeInvalid {
		ep.InvalidReason = strings.TrimSpace(reason)
	}
	if err := os.WriteFile(s.episodePath(ep.ID), []byte(formatEpisodeFile(ep)), 0o644); err != nil {
		return Episode{}, err
	}
	return ep, nil
}

// ReadIDs implements EpisodeIndex.
func (s *EpisodeStore) ReadIDs(_ context.Context, scope identity.TenantScope, ids []string) ([]Episode, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Episode
	seen := map[string]bool{}
	for _, id := range ids {
		id = sanitizeEpisodeID(strings.TrimSpace(id))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ep, err := s.readEpisodeLocked(id)
		if err != nil {
			continue
		}
		if !IsActiveEpisode(ep) {
			continue
		}
		out = append(out, ep)
	}
	return out, nil
}

// IndexText implements EpisodeIndex.
func (s *EpisodeStore) IndexText(_ context.Context, scope identity.TenantScope) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return "", err
	}
	return capIndex(string(data)), nil
}

// ListRecentEpisodes implements EpisodeIndex.
func (s *EpisodeStore) ListRecentEpisodes(ctx context.Context, scope identity.TenantScope, limit int) ([]Episode, error) {
	return s.ListEpisodes(ctx, scope, limit, false)
}

// ListEpisodes lists newest episodes and optionally includes archived/invalid entries.
func (s *EpisodeStore) ListEpisodes(ctx context.Context, scope identity.TenantScope, limit int, includeInactive bool) ([]Episode, error) {
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
	var out []Episode
	for _, e := range entries {
		if len(out) >= limit {
			break
		}
		ep, err := s.readEpisodeLocked(e.ID)
		if err != nil {
			continue
		}
		if !includeInactive && !IsActiveEpisode(ep) {
			continue
		}
		out = append(out, ep)
	}
	return out, nil
}

// Search finds episodes by keyword in id/summary/content.
func (s *EpisodeStore) Search(ctx context.Context, scope identity.TenantScope, q Query) ([]Episode, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 8
	}
	text := strings.ToLower(strings.TrimSpace(q.Text))
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.listIndexEntriesLocked()
	if err != nil {
		return nil, err
	}
	var out []Episode
	for _, e := range entries {
		if len(q.Keys) > 0 && !stringIn(e.ID, q.Keys) && !stringIn(e.LegacyHint, q.Keys) {
			continue
		}
		ep, err := s.readEpisodeLocked(e.ID)
		if err != nil {
			continue
		}
		if !IsActiveEpisode(ep) {
			continue
		}
		if text != "" {
			hay := strings.ToLower(ep.ID + " " + e.Summary + " " + ep.Content + " " + ep.Why + " " + ep.LegacyKey)
			if !strings.Contains(hay, text) && !containsAnyRuneToken(hay, text) {
				continue
			}
		}
		out = append(out, ep)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Write implements Provider via new episodes (legacy key optional in metadata).
func (s *EpisodeStore) Write(ctx context.Context, scope identity.TenantScope, w Write) error {
	meta := w.Metadata
	legacy := strings.TrimSpace(w.Key)
	_, err := s.WriteEpisode(ctx, scope, EpisodeWrite{
		Kind:      EpisodeEvent,
		Content:   w.Content,
		PersonIDs: []string{scope.PersonID()},
		LegacyKey: legacy,
		Metadata:  meta,
	})
	return err
}

// Query implements Provider.
func (s *EpisodeStore) Query(ctx context.Context, scope identity.TenantScope, q Query) ([]Record, error) {
	eps, err := s.Search(ctx, scope, q)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(eps))
	for _, ep := range eps {
		out = append(out, EpisodeToRecord(ep))
	}
	return out, nil
}

// ListRecent implements Provider.
func (s *EpisodeStore) ListRecent(ctx context.Context, scope identity.TenantScope, limit int) ([]Record, error) {
	eps, err := s.ListRecentEpisodes(ctx, scope, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(eps))
	for _, ep := range eps {
		out = append(out, EpisodeToRecord(ep))
	}
	return out, nil
}

type episodeIndexEntry struct {
	ID         string
	Kind       EpisodeKind
	Person     string
	Summary    string
	LegacyHint string
}

func (s *EpisodeStore) listIndexEntriesLocked() ([]episodeIndexEntry, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return nil, err
	}
	var entries []episodeIndexEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		rest := strings.TrimPrefix(line, "- `")
		end := strings.Index(rest, "`")
		if end <= 0 {
			continue
		}
		id := rest[:end]
		rest = strings.TrimSpace(rest[end+1:])
		kind := EpisodeEvent
		person := ""
		summary := rest
		// (kind) [person] summary
		if strings.HasPrefix(rest, "(") {
			close := strings.Index(rest, ")")
			if close > 1 {
				kind = NormalizeEpisodeKind(rest[1:close])
				rest = strings.TrimSpace(rest[close+1:])
			}
		}
		if strings.HasPrefix(rest, "[") {
			close := strings.Index(rest, "]")
			if close > 1 {
				person = strings.TrimSpace(rest[1:close])
				rest = strings.TrimSpace(rest[close+1:])
			}
		}
		summary = strings.TrimPrefix(rest, ":")
		summary = strings.TrimSpace(summary)
		entries = append(entries, episodeIndexEntry{ID: id, Kind: kind, Person: person, Summary: summary})
	}
	return entries, nil
}

func (s *EpisodeStore) prependIndexLocked(ep Episode) error {
	entries, err := s.listIndexEntriesLocked()
	if err != nil {
		return err
	}
	person := ""
	if len(ep.PersonIDs) > 0 {
		person = ep.PersonIDs[0]
	}
	next := []episodeIndexEntry{{
		ID: ep.ID, Kind: ep.Kind, Person: person, Summary: firstLine(ep.Content, 80), LegacyHint: ep.LegacyKey,
	}}
	for _, e := range entries {
		if e.ID == ep.ID {
			continue
		}
		next = append(next, e)
	}
	var b strings.Builder
	b.WriteString("# Episode Index\n\n")
	b.WriteString("<!-- newest first; details in by_id/<id>.md -->\n\n")
	for _, e := range next {
		if e.Person != "" {
			fmt.Fprintf(&b, "- `%s` (%s) [%s]: %s\n", e.ID, e.Kind, e.Person, e.Summary)
		} else {
			fmt.Fprintf(&b, "- `%s` (%s): %s\n", e.ID, e.Kind, e.Summary)
		}
	}
	body := b.String()
	return os.WriteFile(s.indexPath(), []byte(capIndex(body)), 0o644)
}

func (s *EpisodeStore) readEpisodeLocked(id string) (Episode, error) {
	data, err := os.ReadFile(s.episodePath(id))
	if err != nil {
		return Episode{}, err
	}
	return parseEpisodeFile(id, string(data))
}

func formatEpisodeFile(ep Episode) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", ep.ID)
	fmt.Fprintf(&b, "kind: %s\n", ep.Kind)
	status := ep.Status
	if status == "" {
		status = EpisodeActive
	}
	fmt.Fprintf(&b, "status: %s\n", status)
	mode := ep.ExperienceMode
	if mode == "" {
		mode = ExperienceRealInteraction
	}
	fmt.Fprintf(&b, "experience_mode: %s\n", mode)
	fmt.Fprintf(&b, "created_at: %s\n", ep.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "updated_at: %s\n", ep.UpdatedAt.Format(time.RFC3339))
	if ep.SessionID != "" {
		fmt.Fprintf(&b, "session_id: %s\n", ep.SessionID)
	}
	if ep.LegacyKey != "" {
		fmt.Fprintf(&b, "legacy_key: %s\n", ep.LegacyKey)
	}
	if ep.Why != "" {
		fmt.Fprintf(&b, "why: %q\n", ep.Why)
	}
	if ep.InvalidReason != "" {
		fmt.Fprintf(&b, "invalid_reason: %q\n", ep.InvalidReason)
	}
	if len(ep.PersonIDs) > 0 {
		fmt.Fprintf(&b, "person_ids: [%s]\n", strings.Join(quoteList(ep.PersonIDs), ", "))
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(ep.Content))
	b.WriteByte('\n')
	return b.String()
}

func quoteList(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = fmt.Sprintf("%q", x)
	}
	return out
}

func parseEpisodeFile(fallbackID, raw string) (Episode, error) {
	ep := Episode{ID: fallbackID, Kind: EpisodeEvent, Status: EpisodeActive, ExperienceMode: ExperienceRealInteraction}
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		parts := strings.SplitN(raw, "---\n", 3)
		if len(parts) >= 3 {
			for _, line := range strings.Split(parts[1], "\n") {
				line = strings.TrimSpace(line)
				if after, ok := strings.CutPrefix(line, "id:"); ok {
					ep.ID = strings.TrimSpace(after)
				}
				if after, ok := strings.CutPrefix(line, "kind:"); ok {
					ep.Kind = NormalizeEpisodeKind(after)
				}
				if after, ok := strings.CutPrefix(line, "status:"); ok {
					ep.Status = NormalizeEpisodeStatus(after)
				}
				if after, ok := strings.CutPrefix(line, "experience_mode:"); ok {
					ep.ExperienceMode = NormalizeExperienceMode(after)
				}
				if after, ok := strings.CutPrefix(line, "session_id:"); ok {
					ep.SessionID = strings.TrimSpace(after)
				}
				if after, ok := strings.CutPrefix(line, "legacy_key:"); ok {
					ep.LegacyKey = strings.TrimSpace(after)
				}
				if after, ok := strings.CutPrefix(line, "why:"); ok {
					ep.Why = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "invalid_reason:"); ok {
					ep.InvalidReason = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "person_ids:"); ok {
					ep.PersonIDs = parseYAMLStringList(after)
				}
				if after, ok := strings.CutPrefix(line, "created_at:"); ok {
					if t, err := time.Parse(time.RFC3339, strings.TrimSpace(after)); err == nil {
						ep.CreatedAt = t
					}
				}
				if after, ok := strings.CutPrefix(line, "updated_at:"); ok {
					if t, err := time.Parse(time.RFC3339, strings.TrimSpace(after)); err == nil {
						ep.UpdatedAt = t
					}
				}
			}
			body = parts[2]
		}
	}
	ep.Content = strings.TrimSpace(body)
	if ep.ID == "" {
		ep.ID = fallbackID
	}
	if ep.Status == "" {
		ep.Status = EpisodeActive
	}
	if ep.UpdatedAt.IsZero() {
		ep.UpdatedAt = time.Now()
		ep.CreatedAt = ep.UpdatedAt
	}
	if ep.CreatedAt.IsZero() {
		ep.CreatedAt = ep.UpdatedAt
	}
	return ep, nil
}

func parseYAMLStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func sanitizeEpisodeID(id string) string {
	id = strings.TrimSpace(id)
	id = unsafeEpisodeToken.ReplaceAllString(id, "_")
	if len(id) > 80 {
		id = id[:80]
	}
	return id
}
