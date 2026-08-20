// Package room serves the embedded conversation and memory-visualization room.
package room

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"deep-seeing/internal/app"
	"deep-seeing/internal/backup"
	"deep-seeing/internal/graph"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/workspace"
)

const contentTypeHeader = "Content-Type"

//go:embed web/*
var webFS embed.FS

// Server is a loopback web room around one application session.
type Server struct {
	App  *app.App
	Addr string
}

func (s *Server) queue() *runtime.ExecutionQueue {
	if s != nil && s.App != nil && s.App.Queue != nil {
		return s.App.Queue
	}
	return runtime.NewExecutionQueue("room")
}

// Handler returns the room HTTP handler.
func (s *Server) Handler() (http.Handler, error) {
	if s == nil || s.App == nil || s.App.Service == nil {
		return nil, errors.New("room: app required")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/runtime", s.handleRuntime)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/graph", s.handleGraph)
	mux.HandleFunc("GET /api/episodes", s.handleEpisodes)
	mux.HandleFunc("GET /api/episode/{id}", s.handleEpisode)
	mux.HandleFunc("GET /api/proposals", s.handleProposals)
	mux.HandleFunc("GET /api/mutations", s.handleMutations)
	mux.HandleFunc("GET /api/traces", s.handleTraces)
	mux.HandleFunc("GET /api/self", s.handleSelf)
	mux.HandleFunc("GET /api/self/{id}", s.handleSelfArtifact)
	mux.HandleFunc("GET /api/workspace", s.handleWorkspace)
	mux.HandleFunc("GET /api/workspace/{id}", s.handleWorkspaceDocument)
	mux.HandleFunc("GET /api/intents", s.handleIntents)
	mux.HandleFunc("GET /api/wakes", s.handleWakes)
	mux.HandleFunc("GET /api/agency", s.handleAgency)
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("GET /api/source/{id}", s.handleSource)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("POST /api/review", s.handleReview)
	mux.HandleFunc("POST /api/dream", s.handleDream)
	mux.HandleFunc("POST /api/backup", s.handleBackup)

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	serveHTML := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			data, readErr := fs.ReadFile(sub, name)
			if readErr != nil {
				writeError(w, readErr)
				return
			}
			w.Header().Set(contentTypeHeader, "text/html; charset=utf-8")
			_, _ = w.Write(data)
		}
	}
	serveMind := serveHTML("mind.html")
	servePet := serveHTML("pet.html")
	mux.HandleFunc("GET /mind", serveMind)
	mux.HandleFunc("GET /mind/", serveMind)
	mux.HandleFunc("GET /pet", servePet)
	mux.HandleFunc("GET /pet/", servePet)
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return s.securityHeaders(mux), nil
}

// ListenAndServe starts the room.
func (s *Server) ListenAndServe() error {
	h, err := s.Handler()
	if err != nil {
		return err
	}
	addr := strings.TrimSpace(s.Addr)
	if addr == "" {
		addr = "127.0.0.1:3319"
	}
	server := &http.Server{
		Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'")
		if r.Method == http.MethodPost {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			sameOrigin := origin == "" || origin == "http://"+r.Host || origin == "https://"+r.Host
			if !sameOrigin {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin request rejected"})
				return
			}
			if !strings.HasPrefix(strings.ToLower(r.Header.Get(contentTypeHeader)), "application/json") {
				writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "application/json required"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleRuntime(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"runtime":     s.App.RuntimeSnapshot(),
		"graph_label": s.App.GraphLabel,
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	msgs, err := s.App.STM.Get(s.App.SessionID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	if s.App.Graph == nil {
		writeJSON(w, http.StatusOK, graph.View{
			Available: false, Nodes: []graph.ViewNode{}, Edges: []graph.ViewEdge{},
		})
		return
	}
	view, err := s.App.Graph.Visualization(r.Context(), s.App.Scope, queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleEpisodes(w http.ResponseWriter, r *http.Request) {
	eps, err := s.App.Episodes.ListEpisodes(
		r.Context(), s.App.Scope, queryLimit(r, 100), r.URL.Query().Get("all") != "0",
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"episodes": eps})
}

func (s *Server) handleEpisode(w http.ResponseWriter, r *http.Request) {
	ep, err := s.App.Episodes.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "episode not found"})
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (s *Server) handleProposals(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Proposals.ListOpen(r.Context(), s.App.Scope, "", queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposals": items})
}

func (s *Server) handleMutations(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Ledger.ListRecent(queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mutations": items})
}

func (s *Server) handleTraces(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Journal.ListRecent(queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"traces": items})
}

func (s *Server) handleSelf(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Self.ListRecent(queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"artifacts": items})
}

func (s *Server) handleSelfArtifact(w http.ResponseWriter, r *http.Request) {
	item, err := s.App.Self.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "self artifact not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Workspace.List(workspace.ListFilter{Limit: queryLimit(r, 100)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": items})
}

func (s *Server) handleWorkspaceDocument(w http.ResponseWriter, r *http.Request) {
	item, err := s.App.Workspace.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "workspace document not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleIntents(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Intents.ListRecent(r.Context(), s.App.Scope.AgentID, queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"intents": items})
}

func (s *Server) handleWakes(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Intents.ListWakeJobs(r.Context(), "", queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"wakes": items})
}

func (s *Server) handleAgency(w http.ResponseWriter, _ *http.Request) {
	if s.App.Scheduler == nil || s.App.Scheduler.Runner == nil || s.App.Scheduler.Runner.Budget == nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	scheduler := s.App.Scheduler.Snapshot()
	budget := s.App.Scheduler.Runner.Budget.Snapshot(time.Now())
	writeJSON(w, http.StatusOK, map[string]any{
		"available": true,
		"scheduler": scheduler,
		"budget":    budget,
	})
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.World.Sources.ListRecent(queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	type sourceCard struct {
		ID        string    `json:"id"`
		URL       string    `json:"url"`
		FinalURL  string    `json:"final_url,omitempty"`
		Title     string    `json:"title,omitempty"`
		Query     string    `json:"query,omitempty"`
		Excerpt   string    `json:"excerpt,omitempty"`
		FetchedAt time.Time `json:"fetched_at"`
	}
	cards := make([]sourceCard, 0, len(items))
	for _, item := range items {
		cards = append(cards, sourceCard{
			ID: item.ID, URL: item.URL, FinalURL: item.FinalURL, Title: item.Title,
			Query: item.Query, Excerpt: item.Excerpt, FetchedAt: item.FetchedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": cards})
}

func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	item, err := s.App.World.Sources.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "source not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	input.Message = strings.TrimSpace(input.Message)
	if input.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "message required"})
		return
	}

	w.Header().Set(contentTypeHeader, "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stream unsupported"})
		return
	}
	emit := func(kind string, data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"type": kind, "data": data})
		flusher.Flush()
	}
	emit("start", map[string]any{"at": time.Now().Format(time.RFC3339)})
	var answer string
	err := s.queue().RunCognitive(r.Context(), "chat", func(ctx context.Context) error {
		result, err := s.App.Service.StreamTurn(ctx, input.Message,
			func(delta string) { emit("delta", delta) },
			func(name string) { emit("tool", map[string]any{"name": name}) },
		)
		if err != nil {
			return err
		}
		answer = result.Answer
		return nil
	})
	if err != nil {
		log.Printf("chat stream error: %v", err)
		emit("error", map[string]any{"message": err.Error()})
		return
	}
	emit("done", map[string]any{"answer": answer})
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	history, err := s.App.STM.Get(s.App.SessionID)
	if err != nil {
		writeError(w, err)
		return
	}
	var result any
	err = s.queue().RunCognitive(r.Context(), "review", func(ctx context.Context) error {
		res, runErr := s.App.Reviewer.Run(ctx, s.App.Scope, s.App.SessionID, history)
		if runErr != nil {
			return runErr
		}
		result = res
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDream(w http.ResponseWriter, r *http.Request) {
	var result any
	err := s.queue().RunCognitive(r.Context(), "dream", func(ctx context.Context) error {
		res, runErr := s.App.Dreamer.Run(ctx, s.App.Scope, true)
		if runErr != nil {
			return runErr
		}
		result = res
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBackup(w http.ResponseWriter, _ *http.Request) {
	dest, err := backup.Snapshot(".", "", nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": dest})
}

func queryLimit(r *http.Request, fallback int) int {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n <= 0 {
		return fallback
	}
	if n > 500 {
		return 500
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set(contentTypeHeader, "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
}

// ShutdownContext is used by command packages for graceful shutdown.
func ShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
