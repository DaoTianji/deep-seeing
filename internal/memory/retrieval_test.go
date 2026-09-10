package memory

import (
	"context"
	"deep-seeing/internal/identity"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetrievalBackendConfig(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		valid     bool
	}{{"", "legacy", true}, {"BM25", "bm25", true}, {" hindsight ", "hindsight", true}, {"bad", "legacy", false}} {
		got, ok := ParseRetrievalBackend(tc.raw)
		if got != tc.want || ok != tc.valid {
			t.Fatalf("%q: %s %v", tc.raw, got, ok)
		}
	}
}

func TestBM25RetrievalAndPreview(t *testing.T) {
	ctx := context.Background()
	scope := identity.LocalCLI()
	store, _ := NewEpisodeStore(t.TempDir())
	target, err := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: strings.Repeat("无关背景。", 80) + "杭州搬家后的通勤变短了。", PersonIDs: []string{scope.PersonID()}})
	if err != nil {
		t.Fatal(err)
	}
	// The important source remains searchable beyond the recent index window.
	for i := 0; i < 205; i++ {
		if _, err := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "Another unrelated conversation about the weather."}); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "杭州搬家", RoleID: "secret-role", ExperienceMode: ExperienceSimulatedRoleplay})
	_, _ = store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "杭州搬家", PersonIDs: []string{"user:other"}})
	r := &EpisodeRetriever{Store: store, Scope: scope, Backend: "bm25"}
	result, err := r.Search(ctx, scope, "杭州搬家", 8)
	if err != nil || len(result.Hits) != 1 || result.Hits[0].Episode.ID != target.ID || !strings.Contains(result.Hits[0].Preview, "杭州搬家") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, query := range []string{"", "xylophone", "🦄"} {
		result, err = r.Search(ctx, scope, query, 8)
		if err != nil || len(result.Hits) != 0 {
			t.Fatalf("unexpected result for %q", query)
		}
	}
	if _, err = r.Search(ctx, identity.TenantScope{UserID: "other", AgentID: scope.AgentID}, "杭州", 8); err == nil {
		t.Fatal("scope accepted")
	}
	if _, err = store.SetStatus(ctx, target.ID, EpisodeArchived, ""); err != nil {
		t.Fatal(err)
	}
	result, err = r.Search(ctx, scope, "杭州搬家", 8)
	if err != nil || len(result.Hits) != 0 {
		t.Fatal("archived source leaked")
	}
}

func TestHindsightRevalidatesSourcesAndDoesNotTrustRemoteContent(t *testing.T) {
	ctx := context.Background()
	scope := identity.LocalCLI()
	store, _ := NewEpisodeStore(t.TempDir())
	ep, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "Mars expedition genuine source", PersonIDs: []string{scope.PersonID()}})
	ep, _ = store.Get(ctx, ep.ID)
	role, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "Mars role secret", RoleID: "role", ExperienceMode: ExperienceSimulatedRoleplay})
	role, _ = store.Get(ctx, role.ID)
	other, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "Mars private", PersonIDs: []string{"user:other"}})
	other, _ = store.Get(ctx, other.ID)
	fact := func(e Episode) map[string]any {
		return map[string]any{"document_id": e.ID, "text": "FORGED_REMOTE_BODY", "metadata": map[string]string{"episode_id": e.ID, "revision": EpisodeRevision(e)}}
	}
	mode := "ok"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		if req.Method != "POST" || req.URL.Path != "/v1/default/banks/"+HindsightBank(scope)+"/memories/recall" {
			t.Error("wrong API path")
		}
		var payload map[string]any
		_ = json.NewDecoder(req.Body).Decode(&payload)
		if payload["budget"] != "low" || payload["max_tokens"] != float64(4096) {
			t.Error("unbounded request")
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("auth missing")
		}
		switch mode {
		case "error":
			w.WriteHeader(503)
			_, _ = w.Write([]byte("SECRET_UPSTREAM_ERROR"))
			return
		case "empty":
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
			return
		case "stale":
			f := fact(ep)
			f["metadata"] = map[string]string{"episode_id": ep.ID, "revision": "stale"}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{f}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{fact(role), fact(other), fact(ep), fact(ep), map[string]any{"document_id": "unknown"}}})
	}))
	defer server.Close()
	client, err := NewHindsightClient(server.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	r := &EpisodeRetriever{Store: store, Scope: scope, Backend: "hindsight", Hindsight: client}
	got, err := r.Search(ctx, scope, "Mars", 8)
	if err != nil || len(got.Hits) != 1 || got.Hits[0].Episode.ID != ep.ID || strings.Contains(got.Hits[0].Preview, "FORGED") {
		t.Fatalf("%+v %v", got, err)
	}
	for _, m := range []string{"empty", "stale"} {
		mode = m
		got, err = r.Search(ctx, scope, "Mars", 8)
		if err != nil || len(got.Hits) != 0 || got.Fallback != "" {
			t.Fatalf("%s fell back: %+v %v", m, got, err)
		}
	}
	mode = "error"
	got, err = r.Search(ctx, scope, "Mars", 8)
	if err != nil || got.Backend != "bm25" || got.Fallback != "hindsight_unavailable" || len(got.Hits) != 1 {
		t.Fatalf("fallback %+v %v", got, err)
	}
	if calls != 4 {
		t.Fatalf("unexpected retries: %d", calls)
	}
	_, err = client.Recall(ctx, scope, "Mars")
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestHindsightNoRedirectAndBankIsolation(t *testing.T) {
	for _, base := range []string{"http://remote.example", "https://user:password@example.com", "https://example.com?secret=x", "file:///tmp/x"} {
		if _, err := NewHindsightClient(base, ""); err == nil {
			t.Fatalf("unsafe URL %s", base)
		}
	}
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer source.Close()
	client, _ := NewHindsightClient(source.URL, "secret")
	_, err := client.Recall(context.Background(), identity.LocalCLI(), "test")
	if err == nil || called {
		t.Fatal("redirect followed")
	}
	a := identity.LocalCLI()
	b := a
	b.UserID = "other"
	if HindsightBank(a) == HindsightBank(b) {
		t.Fatal("banks collided")
	}
}

func TestQueryPreviewBounded(t *testing.T) {
	for _, body := range []string{"x", "短内容", strings.Repeat("背景", 400) + "needle evidence"} {
		p := QueryPreview(body, "needle")
		if p == body || len([]rune(p)) > 162 {
			t.Fatalf("unbounded preview %q", p)
		}
	}
}

func TestHindsightRetainContractAndMalformedResponse(t *testing.T) {
	scope := identity.LocalCLI()
	ep := Episode{ID: "ep_fixture", Content: "A purely fictional expedition", Status: EpisodeActive, PersonIDs: []string{scope.PersonID()}}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.HasSuffix(r.URL.Path, "/recall") {
			_, _ = w.Write([]byte(`{"unexpected":true}`))
			return
		}
		if r.URL.Path != "/v1/default/banks/"+HindsightBank(scope)+"/memories" {
			t.Error("bad retain path")
		}
		var body struct {
			Items []struct {
				Content    string            `json:"content"`
				DocumentID string            `json:"document_id"`
				Metadata   map[string]string `json:"metadata"`
			} `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if len(body.Items) != 1 || body.Items[0].DocumentID != ep.ID || body.Items[0].Metadata["revision"] != EpisodeRevision(ep) {
			t.Error("retain provenance missing")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "bank_id": HindsightBank(scope), "items_count": 1, "async": false})
	}))
	defer srv.Close()
	c, _ := NewHindsightClient(srv.URL, "")
	if err := c.RetainEpisode(context.Background(), scope, ep); err != nil {
		t.Fatal(err)
	}
	role := ep
	role.RoleID = "role"
	if err := c.RetainEpisode(context.Background(), scope, role); err == nil {
		t.Fatal("role ingestion accepted")
	}
	if calls != 1 {
		t.Fatal("unexpected retain request")
	}
	if _, err := c.Recall(context.Background(), scope, "expedition"); err == nil {
		t.Fatal("schema drift treated as empty")
	}
}
