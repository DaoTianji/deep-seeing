package story

import (
	"context"
	"deep-seeing/internal/world"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func companionFixture(t *testing.T, replies ...string) (*Engine, *scripted, string) {
	t.Helper()
	e, c, o, _ := setup(t, replies...)
	e.Book.Text = "阿青在门前等待朋友。\n\n朋友带来一封信，却还没有打开。\n\n秘密结局：信来自未来。"
	e.Book.Version = "companion-test"
	e.Book.Characters = []Character{{ID: "qing", Name: "阿青"}, {ID: "friend", Name: "朋友"}}
	e.Book.Scenes = []Scene{{ID: 1, Characters: []string{"qing", "friend"}}}
	e.Book.Facts = nil
	e.Book.Excerpts = map[string]Excerpt{}
	return e, c, o
}
func companionRequest() CompanionInput {
	return CompanionInput{RequestID: uuid.NewString(), Speaker: "an", Paragraph: 1, Action: "chat", Message: "这一段意味着什么？"}
}
func TestCompanionParagraphAnchorsEveryBook(t *testing.T) {
	for _, b := range Catalog() {
		e := &Engine{Book: b}
		ps := e.Paragraphs()
		if len(ps) == 0 {
			t.Fatal(b.ID)
		}
		for _, p := range ps {
			if b.Text[p.Start:p.End] != p.Text {
				t.Fatalf("bad source anchor %s %d", b.ID, p.ID)
			}
		}
	}
}

func TestFirstReadingStartsInProseWithoutMovingExistingAnchors(t *testing.T) {
	for _, book := range Catalog() {
		t.Run(book.ID, func(t *testing.T) {
			e := bookEngine(t, book)
			owner := uuid.NewString()
			version := e.textVersion()
			s, err := e.CompanionState(owner)
			if err != nil {
				t.Fatal(err)
			}
			if s.Position.Furthest != 0 || s.Position.Finished {
				t.Fatal("starting at prose pretended reader already read it")
			}
			paragraph := e.Paragraphs()[s.Position.Paragraph-1]
			if book.ReadingStart != "" && !strings.HasPrefix(paragraph.Text, book.ReadingStart) {
				t.Fatal("first page still selects edition front matter")
			}
			if book.Text[paragraph.Start:paragraph.End] != paragraph.Text {
				t.Fatal("source anchor changed")
			}
			// An existing reader may intentionally be at the title page.
			s.Position.Paragraph = 1
			s.Position.Bookmarks = []int{1}
			if err := e.saveCompanion(owner, s); err != nil {
				t.Fatal(err)
			}
			restored, err := e.CompanionState(owner)
			if err != nil || restored.Position.Paragraph != 1 || restored.Position.Bookmarks[0] != 1 || e.textVersion() != version {
				t.Fatal("existing position or text version changed")
			}
		})
	}
}
func TestCompanionReaderPersistenceAndCAS(t *testing.T) {
	e, _, o := companionFixture(t)
	s, err := e.UpdateReader(o, ReaderPosition{Paragraph: 2, Bookmarks: []int{1}, Finished: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.UpdateReader(o, ReaderPosition{Paragraph: 1}, 0); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	s, err = e.AddReaderNote(o, 2, "我的理解", s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := e.companionPath(o)
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("private file permissions")
	}
	restart, _ := New(e.Root, nil, "test", "observe")
	restart.Book = e.Book
	s, err = restart.CompanionState(o)
	if err != nil || len(s.Notes) != 1 || !s.Position.Finished {
		t.Fatal(s, err)
	}
	other, _ := e.CompanionState(uuid.NewString())
	if len(other.Notes) != 0 {
		t.Fatal("visitor leak")
	}
	if _, err = e.CompanionState("../other"); err == nil {
		t.Fatal("unsafe owner")
	}
	e.Book.Text += "新版"
	if _, err = e.CompanionState(o); err == nil {
		t.Fatal("version mismatch accepted")
	}
}
func TestCompanionBoundedContextAndNoStoryWrite(t *testing.T) {
	e, c, o := companionFixture(t, `{"reply":"等待可能体现他的期待。","evidence":[1]}`, `{"ok":true}`)
	b, err := e.Create(o, 1)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := e.path(o, b.ID)
	before, _ := os.ReadFile(path)
	in := companionRequest()
	s, err := e.CompanionTurn(context.Background(), o, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range c.calls {
		if strings.Contains(input, "秘密结局") || strings.Contains(input, "朋友带来") {
			t.Fatal("future leaked", input)
		}
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("companion changed story")
	}
	if len(s.Turns) != 1 || len(c.calls) != 2 {
		t.Fatal("unexpected pipeline")
	}
	if _, err = e.CompanionTurn(context.Background(), o, in, nil); err != nil || len(c.calls) != 2 {
		t.Fatal("retry not idempotent", err)
	}
	in.Message = "不同请求"
	if _, err = e.CompanionTurn(context.Background(), o, in, nil); err == nil {
		t.Fatal("reused id accepted different payload")
	}
}
func TestCompanionFutureHistoryAndOtherSpeakerExcluded(t *testing.T) {
	e, c, o := companionFixture(t, `{"reply":"等待。","evidence":[1]}`, `{"ok":true}`)
	s, _ := e.CompanionState(o)
	s.Turns = []CompanionTurn{{Speaker: "an", Paragraph: 3, Reply: "FUTURE"}, {Speaker: "an", Paragraph: 1, Full: true, Reply: "WHOLE_BOOK"}, {Speaker: "friend", Paragraph: 1, Reply: "OTHER_PERSON"}}
	s.Notes = []CompanionNote{{Paragraph: 3, Text: "SECRET_NOTE"}}
	if err := e.saveCompanion(o, s); err != nil {
		t.Fatal(err)
	}
	if _, err := e.CompanionTurn(context.Background(), o, companionRequest(), nil); err != nil {
		t.Fatal(err)
	}
	for _, input := range c.calls {
		for _, secret := range []string{"FUTURE", "WHOLE_BOOK", "OTHER_PERSON", "SECRET_NOTE"} {
			if strings.Contains(input, secret) {
				t.Fatal("context leak", secret)
			}
		}
	}
	public := s.PublicState()
	if len(public.Turns) != 1 || len(public.Notes) != 0 {
		t.Fatal("visible history filtering")
	}
}
func TestCharacterCompanionNeverResearchesOrGetsAnHistory(t *testing.T) {
	e, c, o := companionFixture(t, `{"items":[{"text":"正在等待朋友","paragraph":1,"kind":"explicit"}]}`, `{"reply":"我希望他能早些到。","evidence":[1]}`, `{"ok":true}`)
	e.Research = func(string) (CompanionResearch, error) { t.Fatal("character tried research"); return nil, nil }
	s, _ := e.CompanionState(o)
	s.Turns = []CompanionTurn{{Speaker: "an", Paragraph: 1, Reply: "PRIVATE_RESEARCH"}}
	_ = e.saveCompanion(o, s)
	in := companionRequest()
	in.Speaker = "qing"
	in.Research = true
	if _, err := e.CompanionTurn(context.Background(), o, in, nil); err != nil {
		t.Fatal(err)
	}
	var actor map[string]any
	_ = json.Unmarshal([]byte(c.calls[1]), &actor)
	if _, ok := actor["visible_text"]; ok {
		t.Fatal("raw omniscient text in actor")
	}
	if strings.Contains(c.calls[1], "PRIVATE_RESEARCH") || !strings.Contains(c.calls[1], "正在等待朋友") {
		t.Fatal("wrong perspective")
	}
}
func TestCompanionFailedGuardNeverPersistsLeakedAnswer(t *testing.T) {
	e, _, o := companionFixture(t, `{"reply":"秘密结局：信来自未来。","evidence":[1]}`, `{"ok":false,"issue":"spoiler"}`)
	s, err := e.CompanionTurn(context.Background(), o, companionRequest(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Turns[0].NeedsSpoiler || strings.Contains(s.Turns[0].Reply, "来自未来") {
		t.Fatal("guard did not contain answer")
	}
	p, _ := e.companionPath(o)
	raw, _ := os.ReadFile(p)
	if strings.Contains(string(raw), "来自未来") {
		t.Fatal("rejected response persisted")
	}
}

func TestCompanionFactsUseOnlyVisibleParagraphReferences(t *testing.T) {
	e, c, o := companionFixture(t, `{"items":[{"text":"等待朋友","paragraph":1,"kind":"explicit"}]}`, `{"reply":"我正等着他。","evidence":[1]}`, `{"ok":true}`)
	ps := e.Paragraphs()
	e.Book.Excerpts = map[string]Excerpt{
		"editor_source": {Text: ps[0].Text, StartByte: ps[0].Start, EndByte: ps[0].End},
		"future_source": {Text: ps[2].Text, StartByte: ps[2].Start, EndByte: ps[2].End},
	}
	e.Book.Facts = []Fact{
		{ID: "editor_fact", Text: "SAFE_WAITING", Audience: []string{"qing"}, EvidenceID: "editor_source"},
		{ID: "future_fact", Text: "FUTURE_SECRET", Audience: []string{"qing"}, EvidenceID: "future_source"},
		{ID: "other_fact", Text: "OTHER_PERSON_SECRET", Audience: []string{"friend"}, EvidenceID: "editor_source"},
	}
	in := companionRequest()
	in.Speaker = "qing"
	if _, err := e.CompanionTurn(context.Background(), o, in, nil); err != nil {
		t.Fatal(err)
	}
	for i, input := range c.calls {
		for _, excluded := range []string{"editor_source", "editor_fact", "FUTURE_SECRET", "OTHER_PERSON_SECRET"} {
			if strings.Contains(input, excluded) {
				t.Fatalf("call %d leaked %s", i, excluded)
			}
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			t.Fatal(err)
		}
		key := "character_facts"
		if i == 0 {
			key = "editorial_facts"
		}
		var facts []companionFact
		if err := json.Unmarshal(payload[key], &facts); err != nil {
			t.Fatal(err)
		}
		if len(facts) != 1 || facts[0].Paragraph != 1 || facts[0].Text != "SAFE_WAITING" {
			t.Fatalf("bad paragraph facts: %+v", facts)
		}
	}
}

func TestCompanionStillRejectsStringCitationIDsWithoutGuessing(t *testing.T) {
	e, _, o := companionFixture(t, `{"items":[]}`, `{"reply":"不能保存","evidence":["e1","f3"]}`)
	in := companionRequest()
	in.Speaker = "qing"
	if _, err := e.CompanionTurn(context.Background(), o, in, nil); err == nil {
		t.Fatal("string IDs were silently accepted")
	}
	s, err := e.CompanionState(o)
	if err != nil || len(s.Turns) != 0 {
		t.Fatal("invalid answer persisted")
	}
}
func TestCompanionTranslationCachesOnlySameStyleAndNeverLoadsOtherText(t *testing.T) {
	e, c, o := companionFixture(t, `{"reply":"A Qing waits by the door."}`)
	in := companionRequest()
	in.Action = "translate"
	in.Style = "literal"
	in.Message = ""
	s, err := e.CompanionTurn(context.Background(), o, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	in.RequestID = uuid.NewString()
	in.Revision = s.Revision
	if _, err = e.CompanionTurn(context.Background(), o, in, nil); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 1 || strings.Contains(c.calls[0], "秘密结局") {
		t.Fatal("translation cache/context")
	}
	in.Style = "fluent"
	if _, err = e.CompanionTurn(context.Background(), o, in, nil); err == nil {
		t.Fatal("unexpected style cache hit")
	}
}

func TestCompanionFailedResearchCannotBeFixedBySpoilerPermission(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprint("full=", full), func(t *testing.T) {
			e, c, o := companionFixture(t, `{"reply":"UNVERIFIED_HISTORY 年法令规定了全部人的命运。","evidence":[1]}`, `{"ok":false,"issue":"unsupported_background"}`)
			in := companionRequest()
			in.Research = true // No gateway: unavailable, including full-book mode.
			if full {
				s, err := e.UpdateReader(o, ReaderPosition{Paragraph: 1, Full: true}, 0)
				if err != nil {
					t.Fatal(err)
				}
				in.Revision = s.Revision
			}
			_, err := e.CompanionTurn(context.Background(), o, in, nil)
			if err == nil || !strings.Contains(err.Error(), "不能替代来源核验") {
				t.Fatalf("wrong failure: %v", err)
			}
			if len(c.calls) != 2 {
				t.Fatal("background guard skipped")
			}
			s, _ := e.CompanionState(o)
			if len(s.Turns) != 0 || s.Position.Full != full {
				t.Fatal("failed evidence check changed state")
			}
		})
	}
}

func TestCompanionUnknownGuardFailureDoesNotInventSpoilerRequirement(t *testing.T) {
	e, _, o := companionFixture(t, `{"reply":"未经核对的内容。"}`, `{"ok":false}`)
	_, err := e.CompanionTurn(context.Background(), o, companionRequest(), nil)
	if err == nil || !strings.Contains(err.Error(), "原因尚不明确") {
		t.Fatalf("wrong error: %v", err)
	}
	s, _ := e.CompanionState(o)
	if len(s.Turns) != 0 || s.Position.Full {
		t.Fatal("unknown rejection changed reading scope")
	}
}

type companionWeb struct {
	queries []string
	reads   []string
	fail    bool
}

func (w *companionWeb) SearchWeb(_ context.Context, q string, _ int) ([]world.SearchHit, world.Source, error) {
	w.queries = append(w.queries, q)
	if w.fail {
		return nil, world.Source{}, errors.New("offline")
	}
	return []world.SearchHit{{URL: "https://example.org/history", Snippet: "UNREAD_SNIPPET"}}, world.Source{}, nil
}
func (w *companionWeb) ReadWebpage(_ context.Context, u string) (world.Source, error) {
	w.reads = append(w.reads, u)
	return world.Source{ID: "read-1", URL: u, Title: "历史背景", Body: "已读公开背景资料"}, nil
}
func TestCompanionResearchRequiresReadAndFiltersRawSource(t *testing.T) {
	e, c, o := companionFixture(t, `{"query":"文学中的书信背景"}`, `{"summary":"信件作为沟通方式。","safe":true,"relevant":true}`, `{"reply":"这里可以从通信背景理解。","evidence":[1]}`, `{"ok":true}`)
	w := &companionWeb{}
	e.Research = func(got string) (CompanionResearch, error) {
		if got != o {
			t.Fatal("wrong source owner")
		}
		return w, nil
	}
	in := companionRequest()
	in.Research = true
	s, err := e.CompanionTurn(context.Background(), o, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.queries) != 1 || len(w.reads) != 1 || len(s.Turns[0].Sources) != 1 {
		t.Fatal("missing read provenance")
	}
	if s.Turns[0].ResearchStatus != "ready" {
		t.Fatal("ready research status missing")
	}
	if strings.Contains(c.calls[2], "UNREAD_SNIPPET") || strings.Contains(c.calls[2], "已读公开背景资料") {
		t.Fatal("raw source escaped card filter")
	}
	var guard struct {
		Sources  []CompanionSource `json:"sources"`
		Question string            `json:"question"`
	}
	if err = json.Unmarshal([]byte(c.calls[3]), &guard); err != nil {
		t.Fatal(err)
	}
	if len(guard.Sources) != 1 || guard.Sources[0].ID != "read-1" || guard.Sources[0].Summary != "信件作为沟通方式。" || guard.Question != in.Message {
		t.Fatal("guard cannot verify the same read background used by the answer")
	}
	if strings.Contains(c.calls[3], "UNREAD_SNIPPET") || strings.Contains(c.calls[3], "已读公开背景资料") {
		t.Fatal("raw search/source leaked into guard")
	}
}
func TestCompanionHTTPReadOnlyAndBookIsolation(t *testing.T) {
	e, _, o := companionFixture(t)
	mux := http.NewServeMux()
	e.registerCatalog(mux)
	get := func(path string) map[string]json.RawMessage {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(&http.Cookie{Name: "reading_visitor", Value: o})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var d map[string]json.RawMessage
		_ = json.Unmarshal(w.Body.Bytes(), &d)
		return d
	}
	_ = get("/api/story/companion?book=magi")
	s, _ := e.CompanionState(o)
	if s.Revision != 0 {
		t.Fatal("GET mutated state")
	}
	if _, err := e.CompanionState("invalid"); err == nil {
		t.Fatal("bad visitor accepted")
	}
}

func TestCompanionUnrelatedReadDoesNotBecomeBackgroundEvidence(t *testing.T) {
	e, c, owner := companionFixture(t, `{"query":"法国女性教育史"}`, `{"summary":"法国电影片单，没有女性教育信息。","safe":true,"relevant":false}`, `{"reply":"没有查到可用的背景资料。","evidence":[1]}`, `{"ok":true}`)
	e.Research = func(string) (CompanionResearch, error) { return &companionWeb{}, nil }
	in := companionRequest()
	in.Research = true
	s, err := e.CompanionTurn(context.Background(), owner, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Turns[0].Sources) != 0 || strings.Contains(c.calls[2], "电影片单") || strings.Contains(c.calls[3], "电影片单") {
		t.Fatal("unrelated read became accepted background")
	}
	if s.Turns[0].ResearchStatus != "no_usable_sources" || !strings.Contains(c.calls[2], "no_usable_sources") {
		t.Fatal("empty research state hidden")
	}
}

func TestCompanionNetworkBudgetSurvivesRestartAndFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.json")
	b := &companionBudget{path: path}
	for i := 0; i < 40; i++ {
		if err := b.reserve(); err != nil {
			t.Fatal(err)
		}
	}
	restart := &companionBudget{path: path}
	if err := restart.reserve(); err == nil {
		t.Fatal("restart bypassed daily budget")
	}
	if err := os.WriteFile(path, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restart.reserve(); err == nil {
		t.Fatal("corrupt budget failed open")
	}
}
