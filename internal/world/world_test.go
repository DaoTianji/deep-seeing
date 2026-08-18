package world_test

import (
	"strings"
	"testing"
	"time"

	"deep-seeing/internal/world"
)

func TestSourceStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := world.NewSourceStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.Save(world.Source{
		URL: "https://example.com/a", Title: "A", Body: "hello body",
		FetchedAt: time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Fatal("expected id")
	}
	got, err := store.Get(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != saved.URL || !strings.Contains(got.Body, "hello body") {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	list, err := store.ListRecent(10)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
}

func TestFetchBudgetExhausts(t *testing.T) {
	b := &world.FetchBudget{MaxPerDay: 2}
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		ok, _ := b.Allow(now)
		if !ok {
			t.Fatalf("allow %d", i)
		}
		b.Consume(now)
	}
	ok, why := b.Allow(now)
	if ok || !strings.Contains(why, "budget") {
		t.Fatalf("expected exhaust, ok=%v why=%q", ok, why)
	}
	next := now.Add(24 * time.Hour)
	ok, _ = b.Allow(next)
	if !ok {
		t.Fatal("next day should allow")
	}
}

func TestExtractTextStripsHTML(t *testing.T) {
	text := world.ExtractText("text/html", []byte(`<html><script>evil()</script><body>Hi <b>there</b></body></html>`))
	if !strings.Contains(text, "Hi") || !strings.Contains(text, "there") {
		t.Fatalf("extract missing text: %q", text)
	}
	if strings.Contains(strings.ToLower(text), "evil") {
		t.Fatalf("script leaked: %q", text)
	}
}
