package story

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func lessonBook(t *testing.T, id string) Book {
	t.Helper()
	for _, b := range ReadingCatalog() {
		if b.ID == id {
			return b
		}
	}
	t.Fatal("missing lesson book", id)
	return Book{}
}

func TestLessonCatalogAndExactParagraphAnchors(t *testing.T) {
	if len(Catalog()) != 5 || len(ReadingCatalog()) != 9 {
		t.Fatal("original stories changed")
	}
	for _, id := range []string{"kong", "beiying", "necklace", "taohuayuan", "quanxue", "mulan"} {
		t.Run(id, func(t *testing.T) {
			e := bookEngine(t, lessonBook(t, id))
			l := e.Lesson()
			if l == nil || len(l.Moments) == 0 || len(l.Sources) < 2 {
				t.Fatal("incomplete lesson")
			}
			for _, m := range l.Moments {
				if m.Paragraph < 1 || e.Paragraphs()[m.Paragraph-1].Text != m.Text || !strings.Contains(m.Text, m.Anchor) {
					t.Fatal("lost original anchor", m.Title)
				}
			}
			for _, v := range l.Views {
				found := false
				for _, c := range e.companionCharacters(l.Moments[0].Paragraph) {
					if c.ID == v.ID {
						found = true
					}
				}
				if !found {
					t.Fatal("unavailable perspective", v.ID)
				}
			}
			for _, s := range l.Sources {
				if !strings.HasPrefix(s.ID, "curated-") || !strings.HasPrefix(s.URL, "https://") {
					t.Fatal("unlabeled source")
				}
			}
		})
	}
}

func TestLessonParagraphsPreserveCompleteOriginal(t *testing.T) {
	for _, id := range []string{"kong", "beiying", "necklace", "taohuayuan", "quanxue", "mulan"} {
		t.Run(id, func(t *testing.T) {
			e := bookEngine(t, lessonBook(t, id))
			var parts []string
			for _, p := range e.Paragraphs() {
				parts = append(parts, p.Text)
				if e.Book.Text[p.Start:p.End] != p.Text {
					t.Fatal("paragraph no longer matches source")
				}
			}
			// Historical English layout includes extra blank lines and indentation.
			if strings.Join(strings.Fields(strings.Join(parts, "\n\n")), " ") != strings.Join(strings.Fields(e.Book.Text), " ") {
				t.Fatal("original truncated or replaced by selected excerpts")
			}
			ending := map[string]string{"kong": "大约孔乙己的确死了", "beiying": "我不知何时再能与他相见", "necklace": "five hundred francs", "taohuayuan": "后遂无问津者", "quanxue": "用心躁也", "mulan": "安能辨我是雄雌"}[id]
			if !strings.Contains(strings.Join(strings.Fields(strings.Join(parts, " ")), " "), ending) {
				t.Fatal("original ending missing")
			}
		})
	}
}

func TestEssayRejectsStoryGenerationCharactersAndTranslation(t *testing.T) {
	e := bookEngine(t, lessonBook(t, "beiying"))
	owner := uuid.NewString()
	c := &scripted{}
	e.Chat = c
	if !e.Book.ReadOnly || len(e.Book.Characters) != 0 || len(e.Book.Scenes) != 0 {
		t.Fatal("essay cast invented")
	}
	if _, err := e.Create(owner, 1); err == nil {
		t.Fatal("essay branch created")
	}
	if err := e.StartReading(owner); err == nil {
		t.Fatal("essay story generation allowed")
	}
	if _, err := e.translationPath(owner, "fluent"); err == nil {
		t.Fatal("Chinese essay treated as an English translation job")
	}
	in := companionRequest()
	in.Speaker = "father"
	if _, err := e.CompanionTurn(context.Background(), owner, in, nil); err == nil {
		t.Fatal("real father impersonation allowed")
	}
	if len(c.calls) != 0 {
		t.Fatal("invalid action called model")
	}
	if len(e.Records(owner)) != 0 {
		t.Fatal("story record created")
	}
}

func TestLessonBackgroundOnlyReachesAnAndHasHonestStatus(t *testing.T) {
	for _, speaker := range []string{"an", "boy"} {
		t.Run(speaker, func(t *testing.T) {
			e := bookEngine(t, lessonBook(t, "kong"))
			c := &scripted{replies: []string{`{"reply":"这里可以从笑声理解人物处境。","evidence":[3]}`, `{"ok":true}`}}
			if speaker != "an" {
				c.replies = append([]string{`{"items":[{"text":"在柜台工作","paragraph":3,"kind":"explicit"}]}`}, c.replies...)
			}
			e.Chat = c
			owner := uuid.NewString()
			s, err := e.UpdateReader(owner, ReaderPosition{Paragraph: 3}, 0)
			if err != nil {
				t.Fatal(err)
			}
			in := companionRequest()
			in.Speaker = speaker
			in.Paragraph = 3
			in.Revision = s.Revision
			out, err := e.CompanionTurn(context.Background(), owner, in, nil)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(c.calls, " ")
			last := out.Turns[len(out.Turns)-1]
			if speaker == "an" {
				if !strings.Contains(joined, "curated_background_notice") || last.ResearchStatus != "curated" || len(last.Sources) < 2 {
					t.Fatal("missing or mislabeled curated context")
				}
			} else if strings.Contains(joined, "curated-") || len(last.Sources) != 0 {
				t.Fatal("background leaked to character")
			}
		})
	}
}

func TestEssayGuardRunsInFullDiscussionAndNotesSurviveRestart(t *testing.T) {
	e := bookEngine(t, lessonBook(t, "beiying"))
	owner := uuid.NewString()
	s, err := e.UpdateReader(owner, ReaderPosition{Paragraph: 5, Full: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	s, err = e.AddReaderNote(owner, 5, "[专题理解 · 动作]\n我注意到了攀这个字。", s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	c := &scripted{replies: []string{`{"reply":"可以注意动作描写，不必要求每个人同样感动。","evidence":[5]}`, `{"ok":true}`}}
	e.Chat = c
	in := companionRequest()
	in.Paragraph = 5
	in.Revision = s.Revision
	if _, err = e.CompanionTurn(context.Background(), owner, in, nil); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 2 || !strings.Contains(c.calls[0], "essay_reading_boundary") {
		t.Fatal("essay boundary or guard missing")
	}
	restart, _ := New(e.Root, nil, "test", "agent")
	restart.Book = e.Book
	s, err = restart.CompanionState(owner)
	if err != nil || len(s.Notes) != 1 {
		t.Fatal("note lost", err)
	}
	other, _ := restart.CompanionState(uuid.NewString())
	if len(other.Notes) != 0 {
		t.Fatal("notes crossed names")
	}
}

func TestLessonHTTPAndLegacyCatalogRoutes(t *testing.T) {
	e := bookEngine(t, Necklace())
	mux := http.NewServeMux()
	e.registerCatalog(mux)
	for _, id := range []string{"kong", "beiying", "necklace", "taohuayuan", "quanxue", "mulan"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/story/lesson?book="+id, nil))
		if w.Code != 200 {
			t.Fatal(id, w.Code, w.Body.String())
		}
		var l ReadingLesson
		if json.Unmarshal(w.Body.Bytes(), &l) != nil || l.ID != id {
			t.Fatal("wrong lesson")
		}
	}
	for _, id := range []string{"necklace", "magi", "leaf", "paw", "kong", "beiying"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/story/book?book="+id, nil))
		if w.Code != 200 {
			t.Fatal("old book route broke", id)
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/story/lesson?book=paw", nil))
	if w.Code != 404 {
		t.Fatal("fake lesson")
	}
}
