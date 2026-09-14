package story

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Names are a convenience selector, NOT proof of identity. Anyone who knows a
// name can open that name's space. Opaque sessions still prevent accidental
// cross-reader requests and never accept a client-supplied filesystem owner.
type Reader struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type readerSession struct {
	ReaderID string    `json:"reader_id"`
	Expires  time.Time `json:"expires"`
}
type readerIndex struct {
	Readers  map[string]Reader        `json:"readers"`
	Sessions map[string]readerSession `json:"sessions"`
}
type readerStore struct {
	mu   sync.Mutex
	root string
}
type readerContextKey struct{}

const readerCookie = "reading_session"
const readerSessionAge = 30 * 24 * time.Hour

func normalizeReaderName(raw string) (name, key string, err error) {
	name = strings.TrimSpace(raw)
	if !utf8.ValidString(raw) || len([]rune(name)) < 1 || len([]rune(name)) > 40 {
		return "", "", errors.New("名字需要1–40个字")
	}
	for _, c := range raw {
		if unicode.IsControl(c) || unicode.Is(unicode.Cf, c) {
			return "", "", errors.New("名字不能包含换行、控制符或不可见格式字符")
		}
	}
	return name, strings.ToLower(name), nil
}
func (s *readerStore) path() string { return filepath.Join(s.root, "readers", "index.json") }
func (s *readerStore) load() (readerIndex, error) {
	d := readerIndex{Readers: map[string]Reader{}, Sessions: map[string]readerSession{}}
	b, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	var loaded readerIndex
	if err = json.Unmarshal(b, &loaded); err != nil {
		return d, err
	}
	d = loaded
	if d.Readers == nil || d.Sessions == nil {
		return d, errors.New("invalid reader index")
	}
	ids := map[string]bool{}
	for key, p := range d.Readers {
		_, canonical, err := normalizeReaderName(p.Name)
		if err != nil || canonical != key || !validID(p.ID) || ids[p.ID] {
			return d, errors.New("invalid reader profile")
		}
		ids[p.ID] = true
	}
	for _, session := range d.Sessions {
		if !ids[session.ReaderID] {
			return d, errors.New("invalid reader session")
		}
	}
	return d, nil
}
func sessionHash(r *http.Request) string {
	c, err := r.Cookie(readerCookie)
	if err != nil || !validID(c.Value) {
		return ""
	}
	h := sha256.Sum256([]byte(c.Value))
	return hex.EncodeToString(h[:])
}
func currentReader(d readerIndex, r *http.Request) *Reader {
	v, ok := d.Sessions[sessionHash(r)]
	if !ok || !time.Now().Before(v.Expires) {
		return nil
	}
	for _, p := range d.Readers {
		if p.ID == v.ReaderID {
			p := p
			return &p
		}
	}
	return nil
}
func (s *readerStore) legacyOwner(d readerIndex, r *http.Request) string {
	c, err := r.Cookie("reading_visitor")
	if err != nil || !validID(c.Value) {
		return ""
	}
	for _, p := range d.Readers {
		if p.ID == c.Value {
			return ""
		}
	}
	paths := []string{filepath.Join(s.root, c.Value), filepath.Join(s.root, "authors", c.Value)}
	for _, b := range Catalog() {
		paths = append(paths, filepath.Join(s.root, "books", b.ID, c.Value))
	}
	for _, p := range paths {
		if stat, err := os.Stat(p); err == nil && stat.IsDir() {
			return c.Value
		}
	}
	return ""
}
func setReaderCookie(w http.ResponseWriter, r *http.Request, token string) {
	maxAge := int(readerSessionAge.Seconds())
	expires := time.Now().Add(readerSessionAge)
	if token == "" {
		maxAge = -1
		expires = time.Unix(1, 0)
	}
	http.SetCookie(w, &http.Cookie{Name: readerCookie, Value: token, Path: "/api/story", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: maxAge, Expires: expires})
}
func readerStoreFailure(w http.ResponseWriter) {
	jsonResponse(w, 503, map[string]string{"error": "名字与会话记录暂不可用，请稍后重试；原存档没有改变"})
}

func (s *readerStore) session(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		readerStoreFailure(w)
		return
	}
	if r.Method == http.MethodGet {
		jsonResponse(w, 200, map[string]any{"reader": currentReader(d, r), "legacy_available": s.legacyOwner(d, r) != ""})
		return
	}
	var in struct {
		Name        string `json:"name"`
		ClaimLegacy bool   `json:"claim_legacy"`
	}
	if !body(w, r, &in) {
		return
	}
	name, key, err := normalizeReaderName(in.Name)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	p, exists := d.Readers[key]
	if in.ClaimLegacy {
		legacy := s.legacyOwner(d, r)
		if exists || legacy == "" {
			jsonResponse(w, 409, map[string]string{"error": "旧匿名记录只能归入一个新名字，不会合并或覆盖已有名字的记录"})
			return
		}
		p = Reader{ID: legacy, Name: name}
	} else if !exists {
		p = Reader{ID: uuid.NewString(), Name: name}
	}
	d.Readers[key] = p
	// Replace only this browser's previous session. Other devices retain theirs.
	delete(d.Sessions, sessionHash(r))
	for key, v := range d.Sessions {
		if !time.Now().Before(v.Expires) {
			delete(d.Sessions, key)
		}
	}
	token := uuid.NewString()
	hash := sha256.Sum256([]byte(token))
	d.Sessions[hex.EncodeToString(hash[:])] = readerSession{p.ID, time.Now().Add(readerSessionAge)}
	if err = atomicJSON(s.path(), d); err != nil {
		readerStoreFailure(w)
		return
	}
	setReaderCookie(w, r, token)
	if in.ClaimLegacy {
		http.SetCookie(w, &http.Cookie{Name: "reading_visitor", Value: "", Path: "/api/story", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	}
	jsonResponse(w, 200, map[string]any{"reader": p, "legacy_available": false})
}
func (s *readerStore) logout(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		readerStoreFailure(w)
		return
	}
	if p := currentReader(d, r); p != nil && r.Header.Get("X-Reading-Profile") != p.ID {
		jsonResponse(w, 409, map[string]string{"error": "当前名字已经变化，请刷新后退出", "code": "reader_changed"})
		return
	}
	delete(d.Sessions, sessionHash(r))
	if err = atomicJSON(s.path(), d); err != nil {
		readerStoreFailure(w)
		return
	}
	setReaderCookie(w, r, "")
	jsonResponse(w, 200, map[string]any{"reader": nil})
}
func (s *readerStore) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/story/session" && (r.Method == http.MethodGet || r.Method == http.MethodPost) {
			s.session(w, r)
			return
		}
		if r.URL.Path == "/api/story/session/logout" && r.Method == http.MethodPost {
			s.logout(w, r)
			return
		}
		s.mu.Lock()
		d, err := s.load()
		var p *Reader
		if err == nil {
			p = currentReader(d, r)
		}
		s.mu.Unlock()
		if err != nil {
			readerStoreFailure(w)
			return
		}
		if p == nil {
			jsonResponse(w, 401, map[string]string{"error": "请先输入名字进入书房", "code": "reader_required"})
			return
		}
		// A cookie is shared by tabs. Bind every request to the profile the page
		// actually displayed so a stale tab cannot write into a newly chosen name.
		if r.Header.Get("X-Reading-Profile") != p.ID {
			jsonResponse(w, 409, map[string]string{"error": "当前名字已经变化，请重新进入书房", "code": "reader_changed"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), readerContextKey{}, p.ID)))
	})
}
