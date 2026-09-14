package story

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteAuthorIsRecoverableAndScoped(t *testing.T) {
	e, _, owner, a := authorFixture(t)
	a = addWork(t, e, owner, a, "待回收的虚构原文。")
	path, _ := e.authorPath(owner, a.ID)
	raw, _ := os.ReadFile(path)
	checkpoint := filepath.Join(filepath.Dir(path), "reads", "checkpoint.json")
	if err := atomicJSON(checkpoint, map[string]string{"test": "saved"}); err != nil {
		t.Fatal(err)
	}
	if err := e.DeleteAuthor(owner, a.ID, a.Revision-1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale delete", err)
	}
	key := "author:" + owner + ":" + a.ID
	e.running[key] = true
	if err := e.DeleteAuthor(owner, a.ID, a.Revision); err == nil {
		t.Fatal("deleted in-flight operation")
	}
	delete(e.running, key)
	if err := e.DeleteAuthor(owner, a.ID, a.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := e.GetAuthor(owner, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("still accessible")
	}
	list, err := e.ListAuthors(owner)
	if err != nil || len(list) != 0 {
		t.Fatal("still listed", err)
	}
	if _, err := e.AddAuthorWork(owner, a.ID, AuthorWork{Title: "late write", Text: "text"}, a.Revision); !errors.Is(err, ErrNotFound) {
		t.Fatal("resurrected deleted author", err)
	}
	archives, err := filepath.Glob(filepath.Join(e.Root, "authors", owner, ".trash", a.ID+"-*"))
	if err != nil || len(archives) != 1 {
		t.Fatal("missing recovery copy", err)
	}
	copy, err := os.ReadFile(filepath.Join(archives[0], "profile.json"))
	if err != nil || string(copy) != string(raw) {
		t.Fatal("recovery changed contents")
	}
	if _, err := os.Stat(filepath.Join(archives[0], "reads", "checkpoint.json")); err != nil {
		t.Fatal("checkpoint lost")
	}
	info, _ := os.Stat(filepath.Dir(archives[0]))
	if info.Mode().Perm() != 0700 {
		t.Fatal("recovery directory not private")
	}
}

func TestDeleteAuthorHTTPIsolationAndConfirmationRevision(t *testing.T) {
	e, _, _, _ := setup(t)
	mux := http.NewServeMux()
	e.Register(mux)
	a := readerTestClient{h: mux}
	b := readerTestClient{h: mux}
	anon := readerTestClient{h: mux}
	a.login(t, "删除验收甲")
	b.login(t, "删除验收乙")
	first, _ := e.CreateAuthor(a.profile.ID, "同名作者", "")
	second, _ := e.CreateAuthor(b.profile.ID, "同名作者", "")
	path := "/api/story/authors/" + first.ID + "/delete"
	if w := anon.call("POST", path, `{"revision":1}`); w.Code != 401 {
		t.Fatal("anonymous delete")
	}
	if w := b.call("POST", path, `{"revision":1}`); w.Code != 404 {
		t.Fatal("other reader delete")
	}
	if w := a.call("POST", path, `{}`); w.Code != 409 {
		t.Fatal("missing revision allowed")
	}
	if w := a.call("POST", path, fmt.Sprintf(`{"revision":%d}`, first.Revision)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := a.call("GET", "/api/story/authors/"+first.ID, ""); w.Code != 404 {
		t.Fatal("old deep link allowed")
	}
	if _, err := e.GetAuthor(b.profile.ID, second.ID); err != nil {
		t.Fatal("other reader affected")
	}
	if w := a.call("GET", "/api/story/books", ""); w.Code != 200 {
		t.Fatal("catalog affected")
	}
}
