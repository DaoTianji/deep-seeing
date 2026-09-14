package story

import (
	"context"
	"github.com/google/uuid"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSimplifiedKongPreservesLegacyReaderAndBranch(t *testing.T) {
	b := lessonBook(t, "kong")
	e := bookEngine(t, b)
	owner := uuid.NewString()
	old, err := libraryFiles.ReadFile("texts/kong-traditional.txt")
	if err != nil {
		t.Fatal(err)
	}
	e.Book.Text = string(old)
	s, err := e.UpdateReader(owner, ReaderPosition{Paragraph: 3, Bookmarks: []int{2}, Full: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	s, err = e.AddReaderNote(owner, 3, "此前的理解", s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	branch, err := e.Create(owner, 2)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := e.companionPath(owner)
	before, _ := os.ReadFile(path)
	e.Book = b
	got, err := e.CompanionState(owner)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version == s.Version || got.Revision != s.Revision || !reflect.DeepEqual(got.Notes, s.Notes) || !reflect.DeepEqual(got.Position, s.Position) {
		t.Fatal("glyph conversion lost reader state")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read silently overwrote old record")
	}
	if _, err = e.Get(owner, branch.ID); err != nil {
		t.Fatal("old story branch lost", err)
	}
	if !strings.Contains(e.Book.Text, "鲁镇") || strings.ContainsAny(e.Book.Text, "魯鎭長讀學淸偸") {
		t.Fatal("traditional display remains")
	}
	if _, err = e.UpdateReader(owner, got.Position, got.Revision); err != nil {
		t.Fatal(err)
	}
	e.Book.Text += "未知编辑"
	if _, err = e.CompanionState(owner); err == nil {
		t.Fatal("unreviewed edit bypassed version check")
	}
}

func TestClassicalModesAndSourceBoundaries(t *testing.T) {
	for _, id := range []string{"taohuayuan", "quanxue", "mulan"} {
		t.Run(id, func(t *testing.T) {
			e := bookEngine(t, lessonBook(t, id))
			owner := uuid.NewString()
			if _, err := e.translationPath(owner, "fluent"); err != nil {
				t.Fatal("classical text must support modern Chinese translation", err)
			}
			if id == "quanxue" {
				if e.Book.TextScope != "课文节选" || len(e.Paragraphs()) != 4 {
					t.Fatal("excerpt scope hidden")
				}
				if _, err := e.Create(owner, 1); err == nil {
					t.Fatal("argument invented story branch")
				}
				return
			}
			branch, err := e.Create(owner, 1)
			if err != nil {
				t.Fatal(err)
			}
			who := "mulan"
			if id == "taohuayuan" {
				who = "fisher"
			}
			snap, err := e.Snapshot(branch, who)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range snap.Facts {
				if f.Since > 1 {
					t.Fatal("future fact leaked")
				}
			}
			if len(snap.Facts) != 1 {
				t.Fatal("missing grounded initial fact")
			}
			if id == "taohuayuan" {
				for _, c := range e.companionCharacters(1) {
					if c.ID == "villager" {
						t.Fatal("unread villager revealed")
					}
				}
				if _, err = e.Snapshot(branch, "villager"); err == nil {
					t.Fatal("offstage character allowed")
				}
			}
		})
	}
}

func TestClassicalCompanionUsesSourcesAndSavesWithoutStoryMutation(t *testing.T) {
	for _, id := range []string{"taohuayuan", "quanxue", "mulan"} {
		t.Run(id, func(t *testing.T) {
			e := bookEngine(t, lessonBook(t, id))
			c := &scripted{replies: []string{`{"reply":"这是一种有待讨论的解读，需结合原文。","evidence":[1]}`, `{"ok":true}`}}
			e.Chat = c
			owner := uuid.NewString()
			in := companionRequest()
			out, err := e.CompanionTurn(context.Background(), owner, in, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Turns) != 1 || len(out.Turns[0].Sources) < 2 || len(e.Records(owner)) != 0 {
				t.Fatal("companion source or isolation failure")
			}
			if !strings.Contains(strings.Join(c.calls, " "), "curated_background_notice") {
				t.Fatal("background missing")
			}
		})
	}
}
