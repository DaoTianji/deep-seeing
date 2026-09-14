package story

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type budgetProbe struct{ calls atomic.Int32 }

func (p *budgetProbe) Complete(context.Context, string, string) (string, error) {
	p.calls.Add(1)
	return "", errors.New("upstream failed")
}
func TestPublicBudgetPersistsAndCountsFailures(t *testing.T) {
	root := t.TempDir()
	p := &budgetProbe{}
	b := &BudgetedCompleter{Next: p, Root: root, DailyLimit: 2}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); b.Complete(context.Background(), "s", "i") }()
	}
	wg.Wait()
	if p.calls.Load() != 2 {
		t.Fatal(p.calls.Load())
	}
	b = &BudgetedCompleter{Next: p, Root: root, DailyLimit: 2}
	b.Complete(context.Background(), "s", "i")
	if p.calls.Load() != 2 {
		t.Fatal("restart reset budget")
	}
	if err := os.WriteFile(filepath.Join(root, "limits", "model-calls.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	b.Complete(context.Background(), "s", "i")
	if p.calls.Load() != 2 {
		t.Fatal("corrupt budget allowed call")
	}
}
func TestPublicGuardCookiesStreamingAndDisabledAuthors(t *testing.T) {
	h := PublicDemoGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "reading_session", Value: "test", HttpOnly: true})
		w.Write([]byte("one"))
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
		w.Write([]byte("two"))
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/story/session", nil))
	if w.Body.String() != "onetwo" || !w.Result().Cookies()[0].Secure || !w.Flushed {
		t.Fatal(w)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/story/authors", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestPublicGuardBoundsWritesAndInflight(t *testing.T) {
	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	h := PublicDemoGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/chat") {
			entered <- struct{}{}
			<-release
		}
		w.WriteHeader(200)
	}))
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/story/companion/chat", nil))
		}()
		<-entered
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/story/companion/chat", nil))
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/story/source", nil))
	if w.Code != 200 {
		t.Fatal("reading blocked")
	}
	close(release)
	wg.Wait()
	for i := 0; i < 121; i++ {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/api/story/session", nil))
	}
	if w.Code != 429 {
		t.Fatal("unlimited writes")
	}
}
