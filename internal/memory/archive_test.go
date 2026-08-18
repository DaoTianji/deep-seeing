package memory_test

import (
	"context"
	"path/filepath"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

func TestArchiveEpisodeHiddenFromSearch(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewEpisodeStore(filepath.Join(dir, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	ep, err := store.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{
		Kind: memory.EpisodeEvent, Content: "一件值得记的事",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetStatus(context.Background(), ep.ID, memory.EpisodeArchived, ""); err != nil {
		t.Fatal(err)
	}
	found, err := store.Search(context.Background(), scope, memory.Query{Text: "值得记", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("archived should be hidden, got %v", found)
	}
	got, err := store.Get(context.Background(), ep.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != memory.EpisodeArchived {
		t.Fatalf("status=%s", got.Status)
	}
}
