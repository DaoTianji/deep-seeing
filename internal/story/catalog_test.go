package story

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func bookEngine(t *testing.T, book Book) *Engine {
	t.Helper()
	e, err := New(t.TempDir(), nil, "test", "agent")
	if err != nil {
		t.Fatal(err)
	}
	e.Book = book
	return e
}

func TestCatalogSourcesAndAllSceneSnapshots(t *testing.T) {
	books := Catalog()
	if len(books) != 5 {
		t.Fatal("expected all five discussed stories")
	}
	seen := map[string]bool{}
	for _, b := range books {
		t.Run(b.ID, func(t *testing.T) {
			if seen[b.ID] {
				t.Fatal("duplicate book")
			}
			seen[b.ID] = true
			if len(b.Text) < 2000 || len(b.Scenes) < 5 || b.EntryScene < 1 || b.EntryScene > len(b.Scenes) {
				t.Fatal("incomplete book")
			}
			chars := map[string]bool{}
			for _, c := range b.Characters {
				chars[c.ID] = true
			}
			for _, f := range b.Facts {
				if _, ok := b.Excerpts[f.EvidenceID]; !ok {
					t.Fatal("missing fact evidence")
				}
				for _, id := range f.Audience {
					if !chars[id] {
						t.Fatal("foreign character")
					}
				}
			}
			e := bookEngine(t, b)
			owner := uuid.NewString()
			for i, scene := range b.Scenes {
				if scene.ID != i+1 {
					t.Fatal("noncontinuous scene")
				}
				for _, id := range scene.EvidenceIDs {
					x, ok := b.Excerpts[id]
					if !ok || x.StartByte < 0 || x.EndByte > len(b.Text) || x.StartByte >= x.EndByte || b.Text[x.StartByte:x.EndByte] != x.Text {
						t.Fatalf("bad excerpt %s", id)
					}
				}
				branch, err := e.Create(owner, scene.ID)
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range scene.Characters {
					if !chars[id] {
						t.Fatal("foreign cast")
					}
					snap, err := e.Snapshot(branch, id)
					if err != nil || len(snap.Facts) == 0 {
						t.Fatalf("empty snapshot %s at %d", id, scene.ID)
					}
					for _, f := range snap.Facts {
						if f.Since > scene.ID || !slices.Contains(f.Audience, id) {
							t.Fatal("knowledge leak")
						}
					}
				}
			}
		})
	}
}

func TestNewStoryKnowledgeBoundaries(t *testing.T) {
	cases := []struct {
		id              string
		scene           int
		char, forbidden string
	}{
		{"magi", 4, "della", "卖掉了自己的怀表"},
		{"leaf", 5, "johnsy", "画下"},
		{"leaf", 5, "sue", "因肺炎去世"},
		{"paw", 2, "herbert", "事故中死亡"},
		{"kong", 3, "boy", "折腿"},
	}
	for _, c := range cases {
		t.Run(c.id+c.char, func(t *testing.T) {
			for _, b := range Catalog() {
				if b.ID != c.id {
					continue
				}
				e := bookEngine(t, b)
				branch, _ := e.Create(uuid.NewString(), c.scene)
				snap, err := e.Snapshot(branch, c.char)
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(snap)
				if strings.Contains(string(raw), c.forbidden) {
					t.Fatal("future/private fact leaked")
				}
			}
		})
	}
}

func TestMultiBookHTTPIsolationAndLegacyStorage(t *testing.T) {
	e, _ := New(t.TempDir(), nil, "test", "agent")
	mux := http.NewServeMux()
	e.registerCatalog(mux)
	owner := uuid.NewString()
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: "reading_visitor", Value: owner})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/story/books", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, b := range Catalog() {
		suffix := "?book=" + b.ID
		created := call("POST", "/api/story/branches"+suffix, `{"scene":1}`)
		if created.Code != 201 {
			t.Fatal(created.Body.String())
		}
		var branch Branch
		_ = json.Unmarshal(created.Body.Bytes(), &branch)
		path := filepath.Join(e.Root, "books", b.ID, owner, branch.ID+".json")
		if b.ID == "necklace" {
			path = filepath.Join(e.Root, owner, branch.ID+".json")
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal("bad storage layout", err)
		}
		if got := call("GET", "/api/story/branches/"+branch.ID+suffix, ""); got.Code != 200 {
			t.Fatal(got.Code)
		}
		other := "necklace"
		if b.ID == other {
			other = "magi"
		}
		if got := call("GET", "/api/story/branches/"+branch.ID+"?book="+other, ""); got.Code != 404 {
			t.Fatal("cross-book read allowed")
		}
		var source struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(call("GET", "/api/story/source"+suffix, "").Body.Bytes(), &source)
		if source.Text != b.Text {
			t.Fatal("wrong book text")
		}
	}
	for _, id := range []string{"unknown", "..%2Fnecklace"} {
		if call("GET", "/api/story/book?book="+id, "").Code != 404 {
			t.Fatal("invalid book allowed")
		}
	}
	if call("GET", "/api/story/book", "").Code != 200 {
		t.Fatal("legacy endpoint broken")
	}
}

func TestEachBookChatAndEndingUseOwnContext(t *testing.T) {
	for _, b := range Catalog() {
		t.Run(b.ID, func(t *testing.T) {
			e := bookEngine(t, b)
			owner := uuid.NewString()
			branch, _ := e.Create(owner, b.EntryScene)
			character := branch.Scene.Characters[0]
			response, _ := json.Marshal(map[string]any{"reply": "我愿意认真想一想，也许还有别的选择。", "evidence_ids": []string{branch.Scene.EvidenceIDs[0]}})
			c := &promptCapture{scripted: scripted{replies: []string{string(response), `{"observation":"人物愿意认真考虑，尚未付诸行动。","changes":[],"events":[],"diverged":false}`, judgeOK}}}
			e.Chat = c
			out, err := e.Turn(context.Background(), owner, branch.ID, character, "先别着急，请想想有没有别的办法。", uuid.NewString(), 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(c.calls[0], "source_url") || strings.Contains(c.calls[0], "guidance") {
				t.Fatal("actor got full book")
			}
			if b.ID != "necklace" && strings.Contains(strings.Join(c.systems, ""), "佛来思节") {
				t.Fatal("necklace prompt contaminated other book")
			}
			if !strings.Contains(c.systems[1], b.Guidance) {
				t.Fatal("missing book boundary")
			}
			d := Decision{Observation: "人物决定暂不仓促行动，故事保留开放的结尾。", NextScene: &out.Scene, Ending: &Ending{Title: "留给明天的选择", Story: "这段相遇没有给所有问题一个确定的答案。人物认真听了来访者的话，没有把陌生人的建议当成命令，而是愿意重新考虑原先的选择。生活仍将继续，这个片刻留下的是多一种可能，而不是已经成功的保证。", Resolution: "以愿意重新考虑选择的开放结尾收束，不冒充行动成功。", NewTimeline: []string{"人物处在原有困境之中。", "交谈后愿意重新考虑，但未宣称问题已经解决。"}, Contributions: []Contribution{}}}
			raw, _ := json.Marshal(d)
			c.replies = []string{string(raw), judgeOK, endingOK}
			closed, err := e.TurnWithOptions(context.Background(), owner, out.ID, character, "", uuid.NewString(), out.Revision, false, true, 8)
			if err != nil || closed.Ending == nil {
				t.Fatal("cannot finish", err)
			}
			fork, err := e.Fork(owner, closed.ID, closed.Revision)
			if err != nil || fork.Ending != nil {
				t.Fatal("cannot fork ending", err)
			}
		})
	}
}
