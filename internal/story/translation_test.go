package story

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func translationReply(from, to int) string {
	items := []map[string]string{}
	for i := from; i <= to; i++ {
		items = append(items, map[string]string{"id": fmt.Sprintf("p%d-s1", i), "translation": fmt.Sprintf("第%d句译文。", i)})
	}
	raw, _ := json.Marshal(map[string]any{"items": items})
	return string(raw)
}
func TestSentenceAnchors(t *testing.T) {
	text := `Mrs. White met Dr. Smith. "Really?" she asked. It cost 3.50 francs. Then he left!`
	ends := sentenceEnds(text)
	if len(ends) != 4 {
		t.Fatalf("wrong boundaries %v", ends)
	}
	for _, book := range Catalog() {
		if book.ID == "kong" {
			continue
		}
		e, _ := New(t.TempDir(), nil, "", "off")
		e.Book = book
		s, err := e.loadTranslation(uuid.NewString(), "fluent")
		if err != nil {
			t.Fatal(err)
		}
		rebuilt := map[int]string{}
		for _, v := range s.Sentences {
			if book.Text[v.Start:v.End] != v.Original {
				t.Fatal("anchor changed")
			}
			rebuilt[v.Paragraph] += v.Original
		}
		for _, p := range e.Paragraphs() {
			if rebuilt[p.ID] != p.Text {
				t.Fatalf("source lost: %s %d", book.ID, p.ID)
			}
		}
	}
}
func TestTranslationCheckpointCacheIsolationAndRestart(t *testing.T) {
	e, c, o, _ := setup(t, translationReply(1, 16), translationReply(17, 18))
	paragraphs := []string{}
	for i := 1; i <= 18; i++ {
		paragraphs = append(paragraphs, fmt.Sprintf("Sentence number %d.", i))
	}
	e.Book.Text = strings.Join(paragraphs, "\n\n")
	before, _ := e.CompanionState(o)
	source := e.Book.Text
	s, err := e.TranslateBatch(context.Background(), o, "fluent", 0)
	if err != nil || s.Completed != 16 {
		t.Fatalf("%+v %v", s, err)
	}
	if _, err = e.TranslateBatch(context.Background(), o, "fluent", 0); err != nil || len(c.calls) != 1 {
		t.Fatal("duplicate request charged twice")
	}
	path := e.sharedTranslationPath("fluent")
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("file permissions")
	}
	other, _ := e.loadTranslation(uuid.NewString(), "fluent")
	literal, _ := e.loadTranslation(o, "literal")
	if other.Completed != 16 || literal.Completed != 0 {
		t.Fatal("default must be shared, styles remain separate")
	}
	restarted, _ := New(e.Root, c, "test", "agent")
	restarted.Book = e.Book
	s, err = restarted.TranslateBatch(context.Background(), o, "fluent", 16)
	if err != nil || s.Completed != 18 {
		t.Fatal(s, err)
	}
	restarted.Chat = nil
	restarted.Mode = "off"
	if _, err = restarted.TranslateBatch(context.Background(), o, "fluent", 18); err != nil || len(c.calls) != 2 {
		t.Fatal("completed cache used model", err)
	}
	after, _ := e.CompanionState(o)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) || e.Book.Text != source {
		t.Fatal("translation changed reader/chat/text")
	}
	restarted.Book.Text += "\n\nNew edition."
	changed, err := restarted.loadTranslation(o, "fluent")
	if err != nil || changed.Completed != 0 {
		t.Fatal("stale edition reused")
	}
}
func TestTranslationInvalidOutputNotSaved(t *testing.T) {
	for _, reply := range []string{`{"items":[]}`, `{"items":[{"id":"p1-s1","translation":"甲"},{"id":"p1-s1","translation":"乙"}]}`, `{"items":[{"id":"wrong","translation":"甲"},{"id":"p2-s1","translation":"乙"}]}`, `{"items":[{"id":"p1-s1","translation":""},{"id":"p2-s1","translation":"乙"}]}`} {
		e, _, o, _ := setup(t, reply)
		e.Book.Text = "Hello.\n\nGoodbye."
		if _, err := e.TranslateBatch(context.Background(), o, "fluent", 0); err == nil {
			t.Fatal("bad output accepted")
		}
		s, err := e.loadTranslation(o, "fluent")
		if err != nil || s.Completed != 0 {
			t.Fatal("partial invalid batch saved")
		}
	}
}
func TestTranslationValidationCancellationAndRecovery(t *testing.T) {
	e, c, o, _ := setup(t, `broken JSON`, translationReply(1, 1))
	e.Book.Text = "Hello."
	if _, err := e.TranslateBatch(context.Background(), o, "../escape", 0); err == nil {
		t.Fatal("invalid style")
	}
	if _, err := e.TranslateBatch(context.Background(), "../escape", "fluent", 0); err == nil {
		t.Fatal("invalid owner")
	}
	if _, err := e.TranslateBatch(context.Background(), o, "fluent", 1); err == nil {
		t.Fatal("future checkpoint")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.TranslateBatch(ctx, o, "fluent", 0); err == nil {
		t.Fatal("cancelled request saved")
	}
	// A cancelled queue request must not call the model.
	if len(c.calls) != 0 {
		t.Fatal("cancelled request charged")
	}
	if _, err := e.TranslateBatch(context.Background(), o, "fluent", 0); err == nil {
		t.Fatal("bad JSON")
	}
	if s, err := e.TranslateBatch(context.Background(), o, "fluent", 0); err != nil || s.Completed != 1 {
		t.Fatal("retry did not recover", err)
	}
	e.Book.ID = "kong"
	if _, err := e.loadTranslation(o, "fluent"); err == nil {
		t.Fatal("Chinese book translated")
	}
}

func TestTranslationPublicRoutesAreReaderAndBookScoped(t *testing.T) {
	e, _, _, _ := setup(t, translationReply(1, 1))
	e.Book.Text = "Hello."
	mux := http.NewServeMux()
	e.Register(mux)
	a := readerTestClient{h: mux}
	b := readerTestClient{h: mux}
	path := "/api/story/companion/translation"
	if w := a.call("GET", path, ""); w.Code != 401 {
		t.Fatal("anonymous translation access")
	}
	if w := a.call("POST", path, `{"style":"fluent","completed":0}`); w.Code != 401 {
		t.Fatal("anonymous generation")
	}
	a.login(t, "译文测试甲")
	b.login(t, "译文测试乙")
	w := a.call("POST", path, `{"style":"fluent","completed":0}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, check := range []struct {
		client    *readerTestClient
		url       string
		completed int
	}{{&a, path, 1}, {&b, path, 1}, {&a, path + "?style=literal", 0}} {
		w = check.client.call("GET", check.url, "")
		var edition TranslationEdition
		if err := json.Unmarshal(w.Body.Bytes(), &edition); err != nil || w.Code != 200 || edition.Completed != check.completed {
			t.Fatal(check.url, w.Code, w.Body.String())
		}
	}
}
