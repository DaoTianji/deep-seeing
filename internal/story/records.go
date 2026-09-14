package story

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type StoryRecord struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Scene     string    `json:"scene"`
	Turns     int       `json:"turns"`
	Completed bool      `json:"completed"`
	Signature string    `json:"signature,omitempty"`
	Updated   time.Time `json:"updated_at"`
}

// Records exposes only metadata belonging to this visitor and this book.
func (e *Engine) Records(owner string) []StoryRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	records := []StoryRecord{}
	if !validID(owner) {
		return records
	}
	entries, err := os.ReadDir(filepath.Join(e.Root, owner))
	if err != nil {
		return records
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !validID(id) {
			continue
		}
		b, err := e.load(owner, id)
		if err != nil {
			continue
		}
		r := StoryRecord{ID: b.ID, Title: b.Scene.Title, Scene: b.Scene.Title, Turns: len(b.Turns), Completed: b.Ending != nil, Signature: b.Signature, Updated: b.Created}
		if len(b.Turns) > 0 {
			r.Updated = b.Turns[len(b.Turns)-1].Created
		}
		if b.Ending != nil {
			r.Title = b.Ending.Title
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Updated.After(records[j].Updated) })
	if len(records) > 50 {
		records = records[:50]
	}
	return records
}

// Signing changes reader-only metadata, not the immutable narrative or turn
// revision. The first signature is final; identical retries are idempotent.
func (e *Engine) Sign(owner, id, name string, revision int) (Branch, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 40 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Branch{}, errors.New("署名需为1至40个字符，不能包含换行或控制字符")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	b, err := e.load(owner, id)
	if err != nil {
		return b, err
	}
	if b.Ending == nil {
		return b, errors.New("故事完结后才能署名")
	}
	if b.Revision != revision {
		return b, ErrConflict
	}
	if b.Signature != "" {
		if b.Signature == name {
			return b, nil
		}
		return b, errors.New("这张卡片已经署名")
	}
	b.Signature = name
	b.SignedAt = time.Now().UTC()
	return b, e.save(b)
}
