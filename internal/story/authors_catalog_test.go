package story

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAuthorCatalogImportPreservesEditionProvenance(t *testing.T) {
	want := map[string][2]string{"necklace": {"translation", "en"}, "magi": {"original", "en"}, "leaf": {"original", "en"}, "paw": {"original", "en"}, "kong": {"original", "zh"}}
	for _, b := range Catalog() {
		t.Run(b.ID, func(t *testing.T) {
			e := bookEngine(t, b)
			o := uuid.NewString()
			a, err := e.CreateAuthor(o, b.Author, "")
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			e.registerAuthors(mux)
			body, _ := json.Marshal(map[string]any{"revision": a.Revision, "book_id": b.ID})
			r := httptest.NewRequest(http.MethodPost, "/api/story/authors/"+a.ID+"/works", bytes.NewReader(body))
			r.AddCookie(&http.Cookie{Name: "reading_visitor", Value: o})
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			a, err = e.GetAuthor(o, a.ID)
			if err != nil || len(a.Works) != 1 {
				t.Fatalf("missing imported work: %v", err)
			}
			work := a.Works[0]
			if work.Kind != want[b.ID][0] || work.Language != want[b.ID][1] {
				t.Errorf("wrong provenance: kind=%s language=%s", work.Kind, work.Language)
			}
			if work.Text != strings.TrimSpace(b.Text) || work.Source != canonicalAuthorURL(b.SourceURL) {
				t.Error("import changed the edition or source")
			}
			if work.Kind == "original" && strings.Contains(work.Attribution, "译本") {
				t.Error("original labelled as translated")
			}
		})
	}
}

func TestAuthorCatalogImportRejectsUnclassifiedEdition(t *testing.T) {
	if _, err := catalogAuthorWork(Book{ID: "unreviewed", Text: "An English sentence."}); err == nil {
		t.Fatal("language was used to guess provenance")
	}
}
