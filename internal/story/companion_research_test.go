package story

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"deep-seeing/internal/world"
)

type researchSequence struct {
	queries, reads     []string
	hits               [][]world.SearchHit
	searchErr, readErr error
}

func (w *researchSequence) SearchWeb(_ context.Context, q string, _ int) ([]world.SearchHit, world.Source, error) {
	w.queries = append(w.queries, q)
	if w.searchErr != nil {
		return nil, world.Source{}, w.searchErr
	}
	if len(w.queries) > len(w.hits) {
		return nil, world.Source{}, nil
	}
	return w.hits[len(w.queries)-1], world.Source{}, nil
}
func (w *researchSequence) ReadWebpage(_ context.Context, u string) (world.Source, error) {
	w.reads = append(w.reads, u)
	return world.Source{ID: fmt.Sprint("read-", len(w.reads)), URL: u, Title: "背景资料", Body: "已读正文"}, w.readErr
}
func quietResearch(string, string) {}

func TestCompanionRephrasesOnceWithoutExtraPlanningOrUnrelatedEvidence(t *testing.T) {
	e, c, owner := companionFixture(t,
		`{"query":"原始背景问题","alternative_query":"同题另一语言"}`,
		`{"summary":"无关电影列表","safe":true,"relevant":false}`,
		`{"summary":"可核实的背景说明","safe":true,"relevant":true}`,
		`{"reply":"结合已读来源理解。","evidence":[1]}`, `{"ok":true}`)
	w := &researchSequence{hits: [][]world.SearchHit{
		{{URL: "https://example.org/unrelated", Snippet: "UNREAD_SNIPPET"}},
		{{URL: "https://example.org/unrelated"}, {URL: "https://example.org/relevant"}},
	}}
	e.Research = func(string) (CompanionResearch, error) { return w, nil }
	in := companionRequest()
	in.Research = true
	var phases []string
	s, err := e.CompanionTurn(context.Background(), owner, in, func(p, m string) { phases = append(phases, p) })
	if err != nil {
		t.Fatal(err)
	}
	if len(w.queries) != 2 || len(w.reads) != 2 || len(c.calls) != 5 || s.Turns[0].ResearchStatus != "ready" || len(s.Turns[0].Sources) != 1 {
		t.Fatalf("wrong bounded flow: queries=%v reads=%v calls=%d state=%+v", w.queries, w.reads, len(c.calls), s)
	}
	if !strings.Contains(strings.Join(phases, ","), "search_rephrased") {
		t.Fatal("retry not visible")
	}
	if !strings.Contains(c.calls[2], "原始背景问题") || strings.Contains(c.calls[3], "无关电影列表") || strings.Contains(c.calls[3], "UNREAD_SNIPPET") {
		t.Fatal("changed relevance target or leaked rejected evidence")
	}
	if len(e.Records(owner)) != 0 {
		t.Fatal("research created story")
	}
	if _, err = e.CompanionTurn(context.Background(), owner, in, nil); err != nil || len(w.queries) != 2 {
		t.Fatal("idempotent request researched twice", err)
	}
}

func TestCompanionResearchStopsOnSuccessOrNetworkFailure(t *testing.T) {
	for _, name := range []string{"success", "search_error", "read_error", "empty", "duplicate_query", "long_alternative", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			e, c, _ := companionFixture(t, `{"summary":"有效背景","safe":true,"relevant":true}`)
			w := &researchSequence{hits: [][]world.SearchHit{{{URL: "https://example.org/1"}}}}
			plan := companionResearchPlan{"original", "alternative"}
			ctx := context.Background()
			wantSearch, wantRead, wantModel := 1, 1, 1
			switch name {
			case "search_error":
				w.searchErr = errors.New("offline")
				wantRead = 0
				wantModel = 0
			case "read_error":
				w.readErr = errors.New("offline")
				wantModel = 0
			case "empty":
				w.hits = nil
				wantSearch = 2
				wantRead = 0
				wantModel = 0
			case "duplicate_query":
				w.hits = nil
				plan.AlternativeQuery = " ORIGINAL "
				wantRead = 0
				wantModel = 0
			case "long_alternative":
				w.hits = nil
				plan.AlternativeQuery = strings.Repeat("长", 181)
				wantRead = 0
				wantModel = 0
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				wantSearch = 0
				wantRead = 0
				wantModel = 0
			}
			_, err := e.collectCompanionSources(ctx, w, plan, quietResearch)
			if (name == "cancelled") != errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if len(w.queries) != wantSearch || len(w.reads) != wantRead || len(c.calls) != wantModel {
				t.Fatalf("unbounded calls queries=%d reads=%d model=%d", len(w.queries), len(w.reads), len(c.calls))
			}
		})
	}
}

func TestCompanionResearchBoundsTotalReads(t *testing.T) {
	reject := `{"summary":"无关","safe":true,"relevant":false}`
	e, _, _ := companionFixture(t, reject, reject, reject, reject)
	w := &researchSequence{hits: [][]world.SearchHit{
		{{URL: "https://example.org/a"}, {URL: "https://example.org/b"}, {URL: "https://example.org/c"}},
		{{URL: "https://example.org/d"}, {URL: "https://example.org/e"}, {URL: "https://example.org/f"}},
	}}
	sources, err := e.collectCompanionSources(context.Background(), w, companionResearchPlan{"a", "b"}, quietResearch)
	if err != nil || len(sources) != 0 || len(w.queries) != 2 || len(w.reads) != 4 {
		t.Fatal("research exceeded bounded attempts", err)
	}
}

func TestCompanionFreeProviderDoesNotUsePresentBraveKey(t *testing.T) {
	t.Setenv("ROLE_SEARCH_PROVIDER", "bing")
	t.Setenv("BRAVE_SEARCH_API_KEY", "unused-test-key")
	if CompanionResearchProviderName() != "bing" {
		t.Fatal("selected paid provider")
	}
	_, _, owner := companionFixture(t)
	r, err := NewCompanionResearchFactory(t.TempDir())(owner)
	if err != nil {
		t.Fatal(err)
	}
	g := r.(*privateCompanionResearch)
	p, ok := g.gateway.Search.(interface{ Name() string })
	if !ok || p.Name() != "bing" {
		t.Fatal("actual gateway differs from reported provider")
	}
}
