package story

import (
	"context"
	"deep-seeing/internal/theater"
	"deep-seeing/internal/world"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type companionBudget struct {
	mu   sync.Mutex
	path string
}

func (b *companionBudget) reserve() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var s struct {
		Day  string `json:"day"`
		Used int    `json:"used"`
	}
	raw, err := os.ReadFile(b.path)
	if err == nil {
		if err = json.Unmarshal(raw, &s); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	today := time.Now().UTC().Format("2006-01-02")
	if s.Day != today {
		s.Day = today
		s.Used = 0
	}
	if s.Used >= 40 {
		return errors.New("今日公开资料额度已用完，明天可继续；仍可阅读和聊天")
	}
	s.Used++
	return atomicJSON(b.path, s)
}

type privateCompanionResearch struct {
	gateway *world.Gateway
	budget  *companionBudget
}

func (g *privateCompanionResearch) seal(src world.Source) error {
	if src.ID == "" {
		return nil
	}
	return os.Chmod(filepath.Join(g.gateway.Sources.Root(), src.ID+".md"), 0600)
}
func (g *privateCompanionResearch) SearchWeb(ctx context.Context, q string, n int) ([]world.SearchHit, world.Source, error) {
	if err := g.budget.reserve(); err != nil {
		return nil, world.Source{}, err
	}
	hits, src, err := g.gateway.SearchWeb(ctx, q, n)
	if err == nil {
		err = g.seal(src)
	}
	return hits, src, err
}
func (g *privateCompanionResearch) ReadWebpage(ctx context.Context, u string) (world.Source, error) {
	if err := g.budget.reserve(); err != nil {
		return world.Source{}, err
	}
	src, err := g.gateway.ReadWebpage(ctx, u)
	if err == nil {
		err = g.seal(src)
	}
	return src, err
}

// CompanionResearchProviderName exposes configuration only, never a secret or
// a claim that the configured search service currently returns useful results.
func CompanionResearchProviderName() string {
	provider, _ := theater.RoleSearchProviderFromEnv(nil)
	return provider.Name()
}

// Sources stay behind each visitor's private directory. The network budget is
// shared across visitors and books; existing URL/DNS/redirect guards apply.
func NewCompanionResearchFactory(root string) func(string) (CompanionResearch, error) {
	var mu sync.Mutex
	gateways := map[string]*privateCompanionResearch{}
	budget := &companionBudget{path: filepath.Join(root, "research", "budget.json")}
	return func(owner string) (CompanionResearch, error) {
		if !validID(owner) {
			return nil, ErrNotFound
		}
		mu.Lock()
		defer mu.Unlock()
		if g := gateways[owner]; g != nil {
			return g, nil
		}
		dir := filepath.Join(root, "research", owner)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
		g, err := world.NewGateway(dir)
		if err != nil {
			return nil, err
		}
		g.Budget = nil // reservation is atomic and survives restarts in this wrapper.
		provider, _ := theater.RoleSearchProviderFromEnv(g)
		g.Search = provider
		private := &privateCompanionResearch{gateway: g, budget: budget}
		gateways[owner] = private
		return private, nil
	}
}
func (e *Engine) registerCompanion(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/story/companion", func(w http.ResponseWriter, r *http.Request) {
		s, err := e.CompanionState(owner(w, r))
		if err != nil {
			failure(w, err)
			return
		}
		// Visible people are keyed to each paragraph so client-side page changes
		// never have to display future catalog identities.
		people := map[int][]Character{}
		for _, p := range e.Paragraphs() {
			people[p.ID] = e.companionCharacters(p.ID)
		}
		provider := ""
		if e.Research != nil {
			provider = e.ResearchProvider
		}
		jsonResponse(w, 200, map[string]any{"state": s.PublicState(), "paragraphs": e.Paragraphs(), "characters": people, "description": e.companionDescription(), "research_available": e.Research != nil, "research_provider": provider})
	})
	mux.HandleFunc("POST /api/story/companion/position", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Position ReaderPosition `json:"position"`
			Revision int            `json:"revision"`
		}
		if !body(w, r, &in) {
			return
		}
		s, err := e.UpdateReader(owner(w, r), in.Position, in.Revision)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, s.PublicState())
	})
	mux.HandleFunc("POST /api/story/companion/notes", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Paragraph int    `json:"paragraph"`
			Text      string `json:"text"`
			Revision  int    `json:"revision"`
		}
		if !body(w, r, &in) {
			return
		}
		s, err := e.AddReaderNote(owner(w, r), in.Paragraph, in.Text, in.Revision)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, s.PublicState())
	})
	mux.HandleFunc("POST /api/story/companion/chat", func(w http.ResponseWriter, r *http.Request) {
		var in CompanionInput
		if !body(w, r, &in) {
			return
		}
		visitor := owner(w, r)
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		enc := json.NewEncoder(w)
		emit := func(kind string, payload any) {
			_ = enc.Encode(map[string]any{"type": kind, "data": payload})
			_ = http.NewResponseController(w).Flush()
		}
		s, err := e.CompanionTurn(r.Context(), visitor, in, func(kind, message string) {
			if kind == "metrics" {
				emit("metrics", json.RawMessage(message))
				return
			}
			emit("status", map[string]string{"phase": kind, "message": message})
		})
		if err != nil {
			emit("error", map[string]string{"message": err.Error()})
			return
		}
		emit("result", s.PublicState())
	})
}
