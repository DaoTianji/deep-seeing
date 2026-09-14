package story

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed texts/*.txt texts/library.json
var libraryFiles embed.FS

// Catalog is editorial source-grounded material, not fabricated AI reading.
func Catalog() []Book {
	var entries []struct {
		Book     Book     `json:"book"`
		Facts    []Fact   `json:"facts"`
		Anchors  []string `json:"anchors"`
		Guidance string   `json:"guidance"`
	}
	raw, err := libraryFiles.ReadFile("texts/library.json")
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(raw, &entries); err != nil {
		panic(err)
	}
	books := []Book{Necklace()}
	for _, entry := range entries {
		b := entry.Book
		raw, err := libraryFiles.ReadFile("texts/" + b.ID + ".txt")
		if err != nil {
			panic(err)
		}
		b.Text = string(raw)
		if b.ID == "kong" {
			// Only this reviewed glyph conversion preserves paragraph identity.
			old, err := libraryFiles.ReadFile("texts/kong-traditional.txt")
			if err != nil {
				panic(err)
			}
			b.CompatibleTextVersions = []string{fmt.Sprintf("%x", sha256.Sum256([]byte(b.Version+string(old))))}
		}
		b.Facts = entry.Facts
		b.Guidance = "\n本作品边界：" + entry.Guidance
		b.Excerpts = map[string]Excerpt{}
		for i, anchor := range entry.Anchors {
			start := strings.Index(b.Text, anchor)
			if start < 0 {
				panic("missing source anchor: " + b.ID)
			}
			b.Excerpts[fmt.Sprintf("e%d", i+1)] = Excerpt{Text: anchor, StartByte: start, EndByte: start + len(anchor)}
		}
		books = append(books, b)
	}
	return books
}

// Keep the original Necklace root intact. Every added book has an allowlisted
// directory and independent state; all books share the same cognitive queue.
func (e *Engine) Register(mux *http.ServeMux) {
	private := http.NewServeMux()
	e.registerCatalog(private)
	store := &readerStore{root: e.Root}
	guard := Middleware(store.protect(private))
	mux.Handle("GET /api/story/", guard)
	mux.Handle("POST /api/story/", guard)
}

// Internal routing remains independently testable; public entry points must
// always use Register, which resolves the named reader before any data access.
func (e *Engine) registerCatalog(mux *http.ServeMux) {
	e.registerAuthors(mux)
	handlers := map[string]http.Handler{}
	engines := map[string]*Engine{}
	books := ReadingCatalog()
	for _, book := range books {
		current := e
		if book.ID != e.Book.ID {
			current = &Engine{Root: filepath.Join(e.Root, "books", book.ID), Book: book, Chat: e.Chat, Model: e.Model, Mode: e.Mode, Provider: e.Provider, Queue: e.Queue, Research: e.Research, ResearchProvider: e.ResearchProvider, running: map[string]bool{}}
		}
		m := http.NewServeMux()
		current.registerBook(m)
		current.registerCompanion(m)
		current.registerLesson(m)
		current.registerTranslation(m)
		handlers[book.ID] = m
		engines[book.ID] = current
	}
	mux.HandleFunc("GET /api/story/books", func(w http.ResponseWriter, r *http.Request) {
		visitor := owner(w, r)
		summaries := []map[string]any{}
		for _, b := range books {
			state, _ := engines[b.ID].CompanionState(visitor)
			summaries = append(summaries, map[string]any{"id": b.ID, "title": b.Title, "author": b.Author, "subtitle": b.Subtitle, "theme": b.Theme, "lesson_id": b.LessonID, "read_only": b.ReadOnly, "text_scope": b.TextScope, "scene_count": len(b.Scenes), "entry_scene": b.EntryScene, "records": engines[b.ID].Records(visitor), "reading": engines[b.ID].Reading(visitor), "reader": state.Position})
		}
		jsonResponse(w, 200, summaries)
	})
	dispatch := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("book")
		if id == "" {
			id = e.Book.ID
		}
		h, ok := handlers[id]
		if !ok {
			jsonResponse(w, 404, map[string]string{"error": "未找到这本书"})
			return
		}
		h.ServeHTTP(w, r)
	})
	mux.Handle("GET /api/story/", dispatch)
	mux.Handle("POST /api/story/", dispatch)
}
