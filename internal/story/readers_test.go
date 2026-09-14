package story

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type readerTestClient struct {
	h       http.Handler
	profile Reader
	cookie  *http.Cookie
}

func (c *readerTestClient) call(method, path, body string, extra ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	if c.profile.ID != "" {
		r.Header.Set("X-Reading-Profile", c.profile.ID)
	}
	for _, cookie := range extra {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	return w
}
func (c *readerTestClient) login(t *testing.T, name string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"name": name})
	w := c.call("POST", "/api/story/session", string(b))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var s struct {
		Reader Reader `json:"reader"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	c.profile = s.Reader
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == readerCookie {
			c.cookie = cookie
		}
	}
	if c.cookie == nil || !c.cookie.HttpOnly || c.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("missing protected session cookie")
	}
}
func readerTestHandler(t *testing.T, root string) http.Handler {
	t.Helper()
	e, err := New(root, nil, "test", "agent")
	if err != nil {
		t.Fatal(err)
	}
	m := http.NewServeMux()
	e.Register(m)
	return m
}

func TestNamedReadersRequireLoginAndIsolateAllRecords(t *testing.T) {
	root := t.TempDir()
	h := readerTestHandler(t, root)
	a := readerTestClient{h: h}
	b := readerTestClient{h: h}
	for _, p := range []string{"/books", "/companion", "/records", "/reading", "/authors", "/source"} {
		if w := a.call("GET", "/api/story"+p, ""); w.Code != 401 {
			t.Fatal("anonymous access", p, w.Code)
		}
	}
	a.login(t, " 小林 ")
	w := a.call("POST", "/api/story/branches", `{"scene":4}`)
	var branch Branch
	_ = json.Unmarshal(w.Body.Bytes(), &branch)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = a.call("POST", "/api/story/companion/notes", `{"paragraph":1,"text":"小林的阅读手记","revision":0}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = a.call("POST", "/api/story/authors", `{"name":"小林的作者档案"}`)
	var author AuthorProfile
	_ = json.Unmarshal(w.Body.Bytes(), &author)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	b.login(t, "小雨")
	if a.profile.ID == b.profile.ID {
		t.Fatal("names merged")
	}
	for _, path := range []string{"/records", "/books", "/companion", "/authors"} {
		w = b.call("GET", "/api/story"+path, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), branch.ID) || strings.Contains(w.Body.String(), "小林的") {
			t.Fatal("other reader data leaked", path, w.Body.String())
		}
	}
	for _, path := range []string{"/branches/" + branch.ID, "/branches/" + branch.ID + "/snapshot?character=mathilde", "/authors/" + author.ID} {
		if w = b.call("GET", "/api/story"+path, ""); w.Code != 404 {
			t.Fatal("guessed ID exposed", w.Code, w.Body.String())
		}
	}
	// Even the known legacy owner ID in a forged cookie cannot override a named session.
	if w = b.call("GET", "/api/story/branches/"+branch.ID, "", &http.Cookie{Name: "reading_visitor", Value: a.profile.ID}); w.Code != 404 {
		t.Fatal("legacy cookie bypass")
	}
	// Existing name on another device resumes the same data, including after restart.
	c := readerTestClient{h: readerTestHandler(t, root)}
	c.login(t, "小林")
	if c.profile.ID != a.profile.ID || c.call("GET", "/api/story/branches/"+branch.ID, "").Code != 200 {
		t.Fatal("same name failed to resume")
	}
	if !strings.Contains(c.call("GET", "/api/story/companion", "").Body.String(), "小林的阅读手记") {
		t.Fatal("notes lost")
	}
	if a.call("GET", "/api/story/records", "").Code != 200 {
		t.Fatal("other device login revoked original")
	}
}

func TestReaderLogoutExpiryAndStaleTabCannotCrossWrite(t *testing.T) {
	root := t.TempDir()
	h := readerTestHandler(t, root)
	c := readerTestClient{h: h}
	c.login(t, "A")
	old := c.cookie
	oldID := c.profile.ID
	c.login(t, "B")
	stale := readerTestClient{h: h, cookie: c.cookie, profile: Reader{ID: oldID}}
	if w := stale.call("POST", "/api/story/companion/notes", `{"paragraph":1,"text":"不能串写","revision":0}`); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := stale.call("POST", "/api/story/session/logout", `{}`); w.Code != 409 {
		t.Fatal("stale tab logged new user out")
	}
	stale.cookie = old
	if w := stale.call("GET", "/api/story/records", ""); w.Code != 401 {
		t.Fatal("replaced token remains valid")
	}
	if w := c.call("POST", "/api/story/session/logout", `{}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := c.call("GET", "/api/story/records", ""); w.Code != 401 {
		t.Fatal("logout token accepted")
	}
	c.login(t, "B")
	store := readerStore{root: root}
	d, err := store.load()
	if err != nil {
		t.Fatal(err)
	}
	for key, s := range d.Sessions {
		s.Expires = time.Now().Add(-time.Minute)
		d.Sessions[key] = s
	}
	if err = atomicJSON(store.path(), d); err != nil {
		t.Fatal(err)
	}
	if w := c.call("GET", "/api/story/records", ""); w.Code != 401 {
		t.Fatal("expired token accepted")
	}
}

func TestReaderNamesValidationPersistenceAndNoPublicDirectory(t *testing.T) {
	root := t.TempDir()
	h := readerTestHandler(t, root)
	c := readerTestClient{h: h}
	for _, name := range []string{"", "  ", strings.Repeat("字", 41), "a\nb", "a\u202eb"} {
		raw, _ := json.Marshal(map[string]string{"name": name})
		if w := c.call("POST", "/api/story/session", string(raw)); w.Code != 400 {
			t.Fatal("bad name accepted", name, w.Code)
		}
	}
	c.login(t, " Alice ")
	id := c.profile.ID
	c.login(t, "alice")
	if c.profile.ID != id {
		t.Fatal("case/trim mismatch")
	}
	raw, err := os.ReadFile(filepath.Join(root, "readers", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), c.cookie.Value) {
		t.Fatal("raw bearer token persisted")
	}
	for path, mode := range map[string]os.FileMode{filepath.Join(root, "readers"): 0700, filepath.Join(root, "readers", "index.json"): 0600} {
		st, err := os.Stat(path)
		if err != nil || st.Mode().Perm() != mode {
			t.Fatal("bad permissions", path, err)
		}
	}
	if w := c.call("GET", "/api/story/readers", ""); w.Code != 404 {
		t.Fatal("reader list exposed")
	}
	if err = atomicJSON(filepath.Join(root, "readers", "index.json"), map[string]string{"bad": "schema"}); err != nil {
		t.Fatal(err)
	}
	if w := c.call("GET", "/api/story/records", ""); w.Code != 503 {
		t.Fatal("corrupt index failed open")
	}
}

func TestLegacyReaderClaimIsExplicitAndNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	e, _ := New(root, nil, "test", "agent")
	legacy := uuid.NewString()
	branch, err := e.Create(legacy, 4)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := e.path(legacy, branch.ID)
	before, _ := os.ReadFile(p)
	m := http.NewServeMux()
	e.Register(m)
	c := readerTestClient{h: m}
	oldCookie := &http.Cookie{Name: "reading_visitor", Value: legacy}
	if w := c.call("GET", "/api/story/records", "", oldCookie); w.Code != 401 {
		t.Fatal("legacy login bypass")
	}
	if w := c.call("GET", "/api/story/session", "", oldCookie); !strings.Contains(w.Body.String(), `"legacy_available":true`) {
		t.Fatal("claim not offered")
	}
	c.login(t, "不导入")
	if c.profile.ID == legacy {
		t.Fatal("automatically claimed")
	}
	w := c.call("POST", "/api/story/session", `{"name":"不导入","claim_legacy":true}`, oldCookie)
	if w.Code != 409 {
		t.Fatal("merged existing profile")
	}
	w = c.call("POST", "/api/story/session", `{"name":"旧书房","claim_legacy":true}`, oldCookie)
	var got struct {
		Reader Reader `json:"reader"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if w.Code != 200 || got.Reader.ID != legacy {
		t.Fatal("claim failed", w.Body.String())
	}
	w = c.call("POST", "/api/story/session", `{"name":"另一个名字","claim_legacy":true}`, oldCookie)
	if w.Code != 409 {
		t.Fatal("same legacy data claimed twice")
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("legacy branch rewritten")
	}
}

func TestReaderConcurrentSameNameAndCrossOrigin(t *testing.T) {
	h := readerTestHandler(t, t.TempDir())
	ids := make(chan string, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := readerTestClient{h: h}
			w := c.call("POST", "/api/story/session", `{"name":"同名"}`)
			var v struct {
				Reader Reader `json:"reader"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &v)
			ids <- v.Reader.ID
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if !validID(id) {
			t.Fatal("login failed")
		}
		if first == "" {
			first = id
		}
		if first != id {
			t.Fatal("same name raced into multiple profiles")
		}
	}
	r := httptest.NewRequest("POST", "/api/story/session", strings.NewReader(`{"name":"evil"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin login")
	}
}
