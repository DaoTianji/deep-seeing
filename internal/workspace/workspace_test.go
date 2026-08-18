package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"deep-seeing/internal/workspace"
)

func TestCreateGetRoundTrip(t *testing.T) {
	store, err := workspace.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := store.Create(workspace.Write{
		Type: workspace.TypeQuestion, Title: "自由意味着什么",
		Body: "还在想：责任是自由的代价还是条件？", Actor: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.ID == "" || d.Status != workspace.StatusOpen || len(d.Revisions) != 1 {
		t.Fatalf("doc=%+v", d)
	}
	got, err := store.Get(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != d.Title || got.Body == "" {
		t.Fatalf("got=%+v", got)
	}
	if _, err := os.Stat(filepath.Join(store.Root(), "questions", d.ID+".md")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAppendsRevision(t *testing.T) {
	store, err := workspace.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := store.Create(workspace.Write{
		Type: workspace.TypeWriting, Title: "草稿", Body: "v1", Actor: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	upd, err := store.Update(d.ID, workspace.Update{
		Body: "v2 续写", RevisionNote: "续写第二段", Actor: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.Revisions) != 2 {
		t.Fatalf("revisions=%v", upd.Revisions)
	}
	if upd.Body != "v2 续写" || upd.Revisions[0].Summary != "created" {
		t.Fatalf("upd=%+v", upd)
	}
}

func TestLinkEpisodeDedup(t *testing.T) {
	store, err := workspace.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := store.Create(workspace.Write{
		Type: workspace.TypeResearch, Title: "调研", Body: "笔记",
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err = store.LinkEpisode(d.ID, "ep_a")
	if err != nil {
		t.Fatal(err)
	}
	d, err = store.LinkEpisode(d.ID, "ep_a")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.EpisodeIDs) != 1 {
		t.Fatalf("episode_ids=%v", d.EpisodeIDs)
	}
	d, err = store.LinkEpisode(d.ID, "ep_b")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.EpisodeIDs) != 2 {
		t.Fatalf("episode_ids=%v", d.EpisodeIDs)
	}
}

func TestListByType(t *testing.T) {
	store, err := workspace.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = store.Create(workspace.Write{Type: workspace.TypeQuestion, Title: "q1", Body: "a"})
	_, _ = store.Create(workspace.Write{Type: workspace.TypeProject, Title: "p1", Body: "b"})
	list, err := store.List(workspace.ListFilter{Type: workspace.TypeQuestion, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Type != workspace.TypeQuestion {
		t.Fatalf("list=%v", list)
	}
}
