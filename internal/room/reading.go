package room

import (
	"deep-seeing/internal/story"
	"io/fs"
	"net/http"
	"os"
)

func (s *Server) registerReadingRoutes(mux *http.ServeMux) error {
	if os.Getenv("STORY_MODE") == "" || os.Getenv("STORY_MODE") == "off" {
		return nil
	}
	root := os.Getenv("STORY_DATA_DIR")
	if root == "" {
		root = "data/reading-showcase"
	}
	var chat story.Completer
	if s.App.RoleArchitect != nil {
		chat = s.App.RoleArchitect.Chat
	}
	e, err := story.New(root, chat, s.App.Model, os.Getenv("STORY_MODE"))
	if err != nil {
		return err
	}
	e.Queue = s.queue()
	e.Register(mux)
	return nil
}

// ReadingHandler serves only the competition APIs and the embedded UI.
// The dedicated process never opens the production app or its memory stores.
func ReadingHandler(engine *story.Engine) (http.Handler, error) {
	mux := http.NewServeMux()
	engine.Register(mux)
	dist, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		return nil, err
	}
	mux.Handle("GET /assets/", http.FileServer(http.FS(dist)))
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reading" && r.URL.Path != "/reading/" {
			http.Redirect(w, r, "/reading", http.StatusFound)
			return
		}
		raw, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "UI build required", 503)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(raw)
	})
	return story.Middleware(mux), nil
}
