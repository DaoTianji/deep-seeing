package room

import (
	"deep-seeing/internal/story"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func readingTestLogin(t *testing.T, h http.Handler) func(*http.Request) {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/story/session", strings.NewReader(`{"name":"路由测试"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var data struct {
		Reader story.Reader `json:"reader"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || w.Code != 200 {
		t.Fatal(w.Body.String(), err)
	}
	cookies := w.Result().Cookies()
	return func(r *http.Request) {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		r.Header.Set("X-Reading-Profile", data.Reader.ID)
	}
}

func TestReadingSearchChannelVisibleAcrossBooksWithoutNetwork(t *testing.T) {
	e, err := story.New(t.TempDir(), nil, "test", "observe")
	if err != nil {
		t.Fatal(err)
	}
	e.ResearchProvider = "bing"
	e.Research = func(string) (story.CompanionResearch, error) {
		t.Fatal("metadata request tried initializing network research")
		return nil, nil
	}
	h, err := ReadingHandler(e)
	if err != nil {
		t.Fatal(err)
	}
	login := readingTestLogin(t, h)
	for _, book := range []string{"necklace", "magi", "leaf", "paw", "kong"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/story/companion?book="+book, nil)
		login(r)
		h.ServeHTTP(w, r)
		var got struct {
			Available bool   `json:"research_available"`
			Provider  string `json:"research_provider"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 || !got.Available || got.Provider != "bing" {
			t.Fatalf("book=%s response=%s err=%v", book, w.Body, err)
		}
	}
}

func TestReadingHandlerMultiBookRoutes(t *testing.T) {
	e, err := story.New(t.TempDir(), nil, "test", "off")
	if err != nil {
		t.Fatal(err)
	}
	h, err := ReadingHandler(e)
	if err != nil {
		t.Fatal(err)
	}
	login := readingTestLogin(t, h)
	for _, path := range []string{"/reading?book=magi", "/api/story/books", "/api/story/book?book=leaf", "/api/story/source?book=kong"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		login(r)
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/story/branches?book=paw", strings.NewReader(`{"scene":2}`))
	r.Header.Set("Content-Type", "application/json")
	login(r)
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/runtime", nil))
	if w.Code != 404 {
		t.Fatal("private app endpoint exposed")
	}
}
