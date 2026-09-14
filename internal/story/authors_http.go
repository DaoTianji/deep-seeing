package story

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func authorBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "请求无效或作品超过1 MiB上传限制"})
		return false
	}
	return true
}
func authorStream(w http.ResponseWriter, run func(func(string)) (AuthorProfile, error)) {
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	enc := json.NewEncoder(w)
	emit := func(kind string, v any) {
		_ = enc.Encode(map[string]any{"type": kind, "data": v})
		_ = http.NewResponseController(w).Flush()
	}
	a, err := run(func(message string) { emit("status", map[string]string{"message": message}) })
	if err != nil {
		emit("error", map[string]string{"message": err.Error()})
		return
	}
	emit("result", a)
}
func (e *Engine) registerAuthors(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/story/authors/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int `json:"revision"`
		}
		if !authorBody(w, r, &in) {
			return
		}
		if err := e.DeleteAuthor(owner(w, r), r.PathValue("id"), in.Revision); err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, map[string]any{"deleted": true, "recoverable": true})
	})
	mux.HandleFunc("POST /api/story/authors/demo", func(w http.ResponseWriter, r *http.Request) {
		a, err := e.CreateAuthorDemo(owner(w, r))
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 201, a)
	})
	mux.HandleFunc("GET /api/story/authors", func(w http.ResponseWriter, r *http.Request) {
		a, err := e.ListAuthors(owner(w, r))
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, a)
	})
	mux.HandleFunc("POST /api/story/authors", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name    string `json:"name"`
			Aliases string `json:"aliases"`
		}
		if !authorBody(w, r, &in) {
			return
		}
		a, err := e.CreateAuthor(owner(w, r), in.Name, in.Aliases)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 201, a)
	})
	mux.HandleFunc("GET /api/story/authors/{id}", func(w http.ResponseWriter, r *http.Request) {
		a, err := e.GetAuthor(owner(w, r), r.PathValue("id"))
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, map[string]any{"author": a, "chunks": authorChunkIndex(a)})
	})
	mux.HandleFunc("POST /api/story/authors/{id}/works", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int        `json:"revision"`
			Work     AuthorWork `json:"work"`
			BookID   string     `json:"book_id"`
			FetchURL bool       `json:"fetch_url"`
		}
		if !authorBody(w, r, &in) {
			return
		}
		visitor := owner(w, r)
		id := r.PathValue("id")
		a, err := e.GetAuthor(visitor, id)
		if err != nil {
			failure(w, err)
			return
		}
		if a.Revision != in.Revision {
			failure(w, ErrConflict)
			return
		}
		if in.BookID != "" {
			found := false
			for _, b := range Catalog() {
				if b.ID == in.BookID {
					if a.Name != b.Author {
						failure(w, errors.New("内置作品的作者姓名与当前档案不一致"))
						return
					}
					in.Work, err = catalogAuthorWork(b)
					if err != nil {
						failure(w, err)
						return
					}
					found = true
					break
				}
			}
			if !found {
				failure(w, ErrNotFound)
				return
			}
		} else if in.FetchURL {
			if in.Work.Private || e.Research == nil {
				failure(w, errors.New("私人资料不自动联网；请粘贴正文，或使用已授权的公开来源"))
				return
			}
			g, err := e.Research(visitor)
			if err != nil {
				failure(w, err)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			src, err := g.ReadWebpage(ctx, in.Work.Source)
			if err != nil {
				failure(w, err)
				return
			}
			in.Work.Text = src.Body
			in.Work.Source = src.URL
			if strings.TrimSpace(in.Work.Title) == "" {
				in.Work.Title = src.Title
			}
		}
		a, err = e.AddAuthorWork(visitor, id, in.Work, in.Revision)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, a)
	})
	mux.HandleFunc("POST /api/story/authors/{id}/search", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Query  string `json:"query"`
			Public bool   `json:"public"`
		}
		if !authorBody(w, r, &in) {
			return
		}
		visitor := owner(w, r)
		a, err := e.GetAuthor(visitor, r.PathValue("id"))
		if err != nil {
			failure(w, err)
			return
		}
		if !in.Public || e.Research == nil || strings.TrimSpace(in.Query) == "" || len([]rune(in.Query)) > 160 {
			failure(w, errors.New("请确认这是公开资料查询，并填写简短搜索词"))
			return
		}
		for _, work := range a.Works {
			if work.Private {
				failure(w, errors.New("含私人作品的档案不向搜索服务发送查询；请直接提供资料"))
				return
			}
		}
		g, err := e.Research(visitor)
		if err != nil {
			failure(w, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		hits, _, err := g.SearchWeb(ctx, a.Name+" "+in.Query, 5)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, hits)
	})
	mux.HandleFunc("POST /api/story/authors/{id}/analyze", func(w http.ResponseWriter, r *http.Request) {
		var in AuthorOperation
		if !authorBody(w, r, &in) {
			return
		}
		visitor := owner(w, r)
		authorStream(w, func(event func(string)) (AuthorProfile, error) {
			return e.AnalyzeAuthor(r.Context(), visitor, r.PathValue("id"), in, event)
		})
	})
	mux.HandleFunc("POST /api/story/authors/{id}/write", func(w http.ResponseWriter, r *http.Request) {
		var in AuthorWritingInput
		if !authorBody(w, r, &in) {
			return
		}
		visitor := owner(w, r)
		authorStream(w, func(event func(string)) (AuthorProfile, error) {
			return e.WriteWithAuthor(r.Context(), visitor, r.PathValue("id"), in, event)
		})
	})
	mux.HandleFunc("POST /api/story/authors/{id}/drafts/{draft}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int    `json:"revision"`
			Text     string `json:"text"`
		}
		if !authorBody(w, r, &in) {
			return
		}
		a, err := e.EditAuthorDraft(owner(w, r), r.PathValue("id"), r.PathValue("draft"), in.Text, in.Revision)
		if err != nil {
			failure(w, err)
			return
		}
		jsonResponse(w, 200, a)
	})
}
