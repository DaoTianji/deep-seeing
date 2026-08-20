package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"deep-seeing/internal/identity"
)

const (
	SceneMaxInjectRunes = 400
	SceneMaxPerTurn     = 2
)

var unsafeSceneToken = regexp.MustCompile(`[^a-zA-Z0-9_\-:]+`)

// SceneNorm is a person-scoped situational norm (not global Bond).
type SceneNorm struct {
	ID        string    `json:"id"`
	PersonID  string    `json:"person_id"`
	Title     string    `json:"title"`
	Keywords  []string  `json:"keywords"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SceneStore persists SceneNorm JSON files under dir/by_person/{person}/.
type SceneStore struct {
	mu  sync.Mutex
	dir string
}

// NewSceneStore opens or creates the scene root.
func NewSceneStore(dir string) (*SceneStore, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "memory", "scenes")
	}
	if err := os.MkdirAll(filepath.Join(dir, "by_person"), 0o755); err != nil {
		return nil, err
	}
	return &SceneStore{dir: dir}, nil
}

// Root returns the store directory.
func (s *SceneStore) Root() string {
	if s == nil {
		return ""
	}
	return s.dir
}

func scenePersonDir(root, personID string) string {
	token := unsafeSceneToken.ReplaceAllString(personID, "_")
	if token == "" {
		token = "unknown"
	}
	return filepath.Join(root, "by_person", token)
}

// Write creates or updates a scene norm. ID may be empty → derived from title.
func (s *SceneStore) Write(scope identity.TenantScope, in SceneNorm) (SceneNorm, error) {
	if s == nil {
		return SceneNorm{}, fmt.Errorf("nil scene store")
	}
	person := strings.TrimSpace(in.PersonID)
	if person == "" {
		person = scope.PersonID()
	}
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = unsafeSceneToken.ReplaceAllString(strings.ToLower(title), "_")
	}
	if id == "" {
		return SceneNorm{}, fmt.Errorf("scene id or title required")
	}
	if title == "" {
		title = id
	}
	kws := normalizeKeywords(in.Keywords)
	if len(kws) == 0 {
		return SceneNorm{}, fmt.Errorf("keywords required for scene bypass recall")
	}
	out := SceneNorm{
		ID: id, PersonID: person, Title: title, Keywords: kws, Body: body,
		UpdatedAt: time.Now().UTC(),
	}
	dir := scenePersonDir(s.dir, person)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SceneNorm{}, err
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return SceneNorm{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(dir, unsafeSceneToken.ReplaceAllString(id, "_")+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return SceneNorm{}, err
	}
	return out, nil
}

// Get loads one scene by id for a person.
func (s *SceneStore) Get(personID, sceneID string) (SceneNorm, error) {
	if s == nil {
		return SceneNorm{}, fmt.Errorf("nil scene store")
	}
	personID = strings.TrimSpace(personID)
	sceneID = strings.TrimSpace(sceneID)
	path := filepath.Join(scenePersonDir(s.dir, personID), unsafeSceneToken.ReplaceAllString(sceneID, "_")+".json")
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(path)
	if err != nil {
		return SceneNorm{}, err
	}
	var out SceneNorm
	if err := json.Unmarshal(raw, &out); err != nil {
		return SceneNorm{}, err
	}
	return out, nil
}

// List returns scenes for a person.
func (s *SceneStore) List(personID string, limit int) ([]SceneNorm, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	dir := scenePersonDir(s.dir, personID)
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SceneNorm
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var sc SceneNorm
		if json.Unmarshal(raw, &sc) != nil {
			continue
		}
		out = append(out, sc)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// MatchQuery returns scenes whose keywords hit the user query (multi-keyword OR).
func (s *SceneStore) MatchQuery(personID, query string, limit int) ([]SceneNorm, error) {
	if limit <= 0 {
		limit = SceneMaxPerTurn
	}
	list, err := s.List(personID, 100)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}
	var out []SceneNorm
	for _, sc := range list {
		if sceneMatches(sc, q) {
			out = append(out, sc)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// FormatSceneRecall renders matched scenes for injection.
func FormatSceneRecall(scenes []SceneNorm) string {
	if len(scenes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Scene norms（场景常模，旁路命中；非全局）:\n")
	used := 0
	for _, sc := range scenes {
		block := fmt.Sprintf("### %s (%s)\nkeywords: %s\n%s\n",
			sc.Title, sc.ID, strings.Join(sc.Keywords, ", "), strings.TrimSpace(sc.Body))
		n := utf8.RuneCountInString(block)
		if used+n > SceneMaxInjectRunes {
			break
		}
		b.WriteString(block)
		used += n
	}
	return strings.TrimSpace(b.String())
}

func normalizeKeywords(kws []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range kws {
		k = strings.ToLower(strings.TrimSpace(k))
		if utf8.RuneCountInString(k) < 2 || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func sceneMatches(sc SceneNorm, queryLower string) bool {
	for _, kw := range sc.Keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw != "" && strings.Contains(queryLower, kw) {
			return true
		}
	}
	return false
}
