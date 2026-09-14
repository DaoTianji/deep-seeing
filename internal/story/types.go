// Package story implements the isolated reading competition prototype.
package story

import (
	"context"
	"time"
)

type Completer interface {
	Complete(context.Context, string, string) (string, error)
}
type Character struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Initial     string `json:"initial"`
	Description string `json:"description"`
	Color       string `json:"color"`
}
type Evidence struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Note   string `json:"note"`
	Scene  int    `json:"scene"`
	Origin string `json:"origin"`
}
type Fact struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Since      int      `json:"since"`
	Audience   []string `json:"audience"`
	EvidenceID string   `json:"evidence_id"`
}
type Scene struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Time        string   `json:"time"`
	Place       string   `json:"place"`
	Summary     string   `json:"summary"`
	Question    string   `json:"question"`
	Choice      string   `json:"choice"`
	Resistance  string   `json:"resistance"`
	Characters  []string `json:"characters"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type Book struct {
	LessonID               string             `json:"lesson_id,omitempty"`
	TextScope              string             `json:"text_scope,omitempty"`
	CompatibleTextVersions []string           `json:"-"`
	ReadOnly               bool               `json:"read_only,omitempty"`
	Language               string             `json:"language,omitempty"`
	ReadingStart           string             `json:"reading_start,omitempty"`
	Text                   string             `json:"-"`
	EntryScene             int                `json:"entry_scene"`
	Theme                  string             `json:"theme"`
	Guidance               string             `json:"-"`
	Excerpts               map[string]Excerpt `json:"excerpts"`
	ID                     string             `json:"id"`
	Title                  string             `json:"title"`
	Author                 string             `json:"author"`
	Subtitle               string             `json:"subtitle"`
	SourceURL              string             `json:"source_url"`
	SourceNote             string             `json:"source_note"`
	Version                string             `json:"version"`
	Characters             []Character        `json:"characters"`
	Scenes                 []Scene            `json:"scenes"`
	Evidence               []Evidence         `json:"evidence"`
	Facts                  []Fact             `json:"-"`
}
type Excerpt struct {
	Text      string `json:"text"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
}
type Insight struct {
	Scene       int      `json:"scene"`
	Observation string   `json:"observation"`
	Question    string   `json:"question"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type Reading struct {
	Status     string    `json:"status"`
	Completed  int       `json:"completed"`
	Insights   []Insight `json:"insights"`
	Error      string    `json:"error,omitempty"`
	Updated    time.Time `json:"updated_at"`
	SourceHash string    `json:"source_hash"`
	Model      string    `json:"model"`
}
type Memory struct {
	ID          string `json:"id"`
	CharacterID string `json:"character_id"`
	Text        string `json:"text"`
	Kind        string `json:"kind"`
	Revision    int    `json:"revision"`
	TurnID      string `json:"turn_id"`
}
type Turn struct {
	Influence   int       `json:"influence,omitempty"`
	Kind        string    `json:"kind"`
	ID          string    `json:"id"`
	RequestID   string    `json:"request_id"`
	CharacterID string    `json:"character_id"`
	Message     string    `json:"message"`
	Reply       string    `json:"reply"`
	Observation string    `json:"observation"`
	EvidenceIDs []string  `json:"evidence_ids"`
	Changes     []Memory  `json:"changes"`
	SceneTitle  string    `json:"scene_title"`
	Revision    int       `json:"revision"`
	Created     time.Time `json:"created_at"`
}
type Branch struct {
	Signature      string    `json:"signature,omitempty"`
	SignedAt       time.Time `json:"signed_at,omitempty"`
	Influence      int       `json:"influence"`
	Ending         *Ending   `json:"ending,omitempty"`
	ID             string    `json:"id"`
	Owner          string    `json:"-"`
	BookVersion    string    `json:"book_version"`
	ParentID       string    `json:"parent_id,omitempty"`
	ParentRevision int       `json:"parent_revision,omitempty"`
	Anchor         int       `json:"anchor"`
	Revision       int       `json:"revision"`
	Scene          Scene     `json:"scene"`
	Memories       []Memory  `json:"memories"`
	Turns          []Turn    `json:"turns"`
	Events         []string  `json:"events"`
	Diverged       bool      `json:"diverged"`
	Created        time.Time `json:"created_at"`
}
type Snapshot struct {
	Character Character `json:"character"`
	Place     string    `json:"place"`
	Scene     string    `json:"scene"`
	Situation string    `json:"situation"`
	Facts     []Fact    `json:"facts"`
	Memories  []Memory  `json:"memories"`
	History   []Turn    `json:"history"`
}
type Decision struct {
	Ending      *Ending  `json:"ending,omitempty"`
	Observation string   `json:"observation"`
	Changes     []Memory `json:"changes"`
	Events      []string `json:"events"`
	Diverged    bool     `json:"diverged"`
	NextScene   *Scene   `json:"next_scene,omitempty"`
}

// Ending is a reader-only artifact. It never enters a character's snapshot.
type Ending struct {
	Title         string         `json:"title"`
	Story         string         `json:"story"`
	Resolution    string         `json:"resolution"`
	NewTimeline   []string       `json:"new_timeline"`
	Contributions []Contribution `json:"contributions"`
	Revision      int            `json:"revision"`
	Created       time.Time      `json:"created_at"`
}
type Contribution struct {
	TurnID string `json:"turn_id"`
	Text   string `json:"text"`
}
