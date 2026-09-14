package story

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
)

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func owner(w http.ResponseWriter, r *http.Request) string {
	if id, ok := r.Context().Value(readerContextKey{}).(string); ok {
		return id
	}
	if c, err := r.Cookie("reading_visitor"); err == nil && validID(c.Value) {
		return c.Value
	}
	id := uuid.NewString()
	http.SetCookie(w, &http.Cookie{Name: "reading_visitor", Value: id, Path: "/api/story", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: 60 * 60 * 24 * 30})
	return id
}
func body(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "请求内容无效"})
		return false
	}
	return true
}
func failure(w http.ResponseWriter, err error) {
	status := 422
	if errors.Is(err, ErrNotFound) {
		status = 404
	}
	if errors.Is(err, ErrConflict) {
		status = 409
	}
	jsonResponse(w, status, map[string]string{"error": err.Error()})
}

func (e *Engine) registerBook(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/story/records", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, e.Records(owner(w, r))) })
	mux.HandleFunc("POST /api/story/branches/{id}/signature", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name     string `json:"name"`
			Revision int    `json:"revision"`
		}
		if !body(w, r, &in) {
			return
		}
		b, err := e.Sign(owner(w, r), r.PathValue("id"), in.Name, in.Revision)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, b)
	})
	mux.HandleFunc("GET /api/story/book", func(w http.ResponseWriter, r *http.Request) {
		o := owner(w, r)
		jsonResponse(w, 200, map[string]any{"book": e.Book, "reading": e.Reading(o), "model_connected": e.Chat != nil, "mode": e.Mode, "provider": e.Provider, "model": e.Model})
	})
	mux.HandleFunc("GET /api/story/source", func(w http.ResponseWriter, r *http.Request) {
		_ = owner(w, r)
		jsonResponse(w, 200, map[string]any{"text": e.Book.Text, "url": e.Book.SourceURL, "label": "历史原文 · 包含完整结局"})
	})
	mux.HandleFunc("GET /api/story/reading", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, e.Reading(owner(w, r))) })
	mux.HandleFunc("POST /api/story/reading", func(w http.ResponseWriter, r *http.Request) {
		o := owner(w, r)
		if err := e.StartReading(o); err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 202, e.Reading(o))
	})
	mux.HandleFunc("POST /api/story/branches", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Scene int `json:"scene"`
		}
		if !body(w, r, &in) {
			return
		}
		b, err := e.Create(owner(w, r), in.Scene)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 201, b)
	})
	mux.HandleFunc("GET /api/story/branches/{id}", func(w http.ResponseWriter, r *http.Request) {
		b, err := e.Get(owner(w, r), r.PathValue("id"))
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, b)
	})
	mux.HandleFunc("GET /api/story/branches/{id}/snapshot", func(w http.ResponseWriter, r *http.Request) {
		b, err := e.Get(owner(w, r), r.PathValue("id"))
		if err != nil {
			failure(w, err)
			return
		}
		s, err := e.Snapshot(b, r.URL.Query().Get("character"))
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, s)
	})
	mux.HandleFunc("POST /api/story/branches/{id}/fork", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int `json:"revision"`
		}
		if !body(w, r, &in) {
			return
		}
		b, err := e.Fork(owner(w, r), r.PathValue("id"), in.Revision)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 201, b)
	})
	mux.HandleFunc("POST /api/story/branches/{id}/turn", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Character string `json:"character"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
			Revision  int    `json:"revision"`
			Advance   bool   `json:"advance"`
			Finish    bool   `json:"finish"`
			Influence int    `json:"influence"`
		}
		if !body(w, r, &in) {
			return
		}
		b, err := e.TurnWithOptions(r.Context(), owner(w, r), r.PathValue("id"), in.Character, in.Message, in.RequestID, in.Revision, in.Advance, in.Finish, in.Influence)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, b)
	})
}

// Middleware is also used by the isolated competition server. It exposes no
// ordinary Room, Soul, Bond, or other roles. Cookies scope the visitor's files.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; img-src 'self' data:; connect-src 'self'")
		if r.Method == http.MethodPost {
			origin := r.Header.Get("Origin")
			if origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
				jsonResponse(w, 403, map[string]string{"error": "跨站请求已拒绝"})
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				jsonResponse(w, 415, map[string]string{"error": "需要JSON请求"})
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/story") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// Kept here to document the asynchronous reading checkpoint cadence.
const PollInterval = 2 * time.Second
