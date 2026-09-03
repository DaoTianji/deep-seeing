package theater

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestCorpusIngestSearchDeduplicateAndRebuild(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	corpus, err := NewCorpusStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	text := "# 生平\n\n林舟在雾港长大，学习潮汐。\n\n# 争议\n\n白鸟事故原因没有定论。"
	document, chunks, duplicate, err := corpus.Ingest(ctx, CorpusIngestInput{
		RoleID: "role_1", CorpusRoleID: "role_1", SourceID: "rsrc_1",
		Title: "人物小传", Audience: SourceActor, Tier: SourceBiography, Text: []byte(text),
	})
	if err != nil || duplicate || document.ChunkCount == 0 || len(chunks) == 0 {
		t.Fatalf("ingest: %#v %#v %v %v", document, chunks, duplicate, err)
	}
	again, _, duplicate, err := corpus.Ingest(ctx, CorpusIngestInput{
		RoleID: "role_1", CorpusRoleID: "role_1", SourceID: "rsrc_2",
		Title: "重复小传", Audience: SourceActor, Tier: SourceBiography, Text: []byte(text),
	})
	if err != nil || !duplicate || again.ID != document.ID {
		t.Fatalf("deduplicate: %#v %v %v", again, duplicate, err)
	}
	cards, err := corpus.Search(ctx, "role_1", "白鸟 事故", SourceActor, 10)
	if err != nil || len(cards) == 0 || strings.Contains(cards[0].Excerpt, "林舟在雾港") {
		t.Fatalf("search: %#v %v", cards, err)
	}
	chunk, err := corpus.ReadChunk(ctx, cards[0].ID)
	if err != nil || !strings.Contains(chunk.Content, "白鸟") {
		t.Fatalf("read: %#v %v", chunk, err)
	}
	if err := corpus.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	cards, err = corpus.Search(ctx, "role_1", "潮汐", SourceActor, 10)
	if err != nil || len(cards) == 0 {
		t.Fatalf("search after rebuild: %#v %v", cards, err)
	}
	info, err := os.Stat(root + "/corpus-index.db")
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("db permissions: %#v %v", info, err)
	}
}

func TestCorpusAudienceAndSizeBoundary(t *testing.T) {
	ctx := context.Background()
	corpus, err := NewCorpusStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	_, _, _, err = corpus.Ingest(ctx, CorpusIngestInput{RoleID: "r", Title: "bad", Text: []byte{0xff}})
	if err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	long := strings.Repeat("甲", MaxRoleChunkRunes+100)
	_, chunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: "r", Title: "long", Audience: SourceDirector, Tier: SourcePosthumous, Text: []byte(long)})
	if err != nil || len(chunks) < 2 {
		t.Fatalf("long chunking: %d %v", len(chunks), err)
	}
	cards, err := corpus.Search(ctx, "r", "甲", SourceActor, 10)
	if err != nil || len(cards) != 0 {
		t.Fatalf("director source leaked: %#v %v", cards, err)
	}
	cards, err = corpus.Search(ctx, "r", "甲", SourceDirector, 10)
	if err != nil || len(cards) == 0 {
		t.Fatalf("director source missing: %#v %v", cards, err)
	}
}

func TestCorpusCanonicalURLDeduplication(t *testing.T) {
	ctx := context.Background()
	corpus, err := NewCorpusStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	first, _, duplicate, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: "role-url", SourceID: "source-1", SourceURL: "HTTPS://Example.com/adler/#life", Title: "first", Audience: SourceActor, Tier: SourceBiography, Text: []byte("first body")})
	if err != nil || duplicate {
		t.Fatalf("first ingest: duplicate=%v err=%v", duplicate, err)
	}
	second, _, duplicate, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: "role-url", SourceID: "source-2", SourceURL: "https://example.com/adler", Title: "duplicate", Audience: SourceActor, Tier: SourceBiography, Text: []byte("changed mirror body")})
	if err != nil || !duplicate || second.ID != first.ID || second.SourceID != "source-1" {
		t.Fatalf("URL dedupe failed: %#v %#v duplicate=%v err=%v", first, second, duplicate, err)
	}
}

func TestCorpusRestoresLegacyChunkOrderFromSourceText(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	corpus, err := NewCorpusStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	text := strings.Repeat("甲", 3900) + "\n\n" + strings.Repeat("乙", 3900) + "\n\n" + strings.Repeat("丙", 3900)
	document, created, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: "legacy-role", Title: "旧书", Audience: SourceActor, Tier: SourcePrimary, Text: []byte(text)})
	if err != nil || len(created) < 3 {
		t.Fatalf("ingest chunks=%d err=%v", len(created), err)
	}
	for _, chunk := range created {
		chunk.Sequence = 0
		path := root + "/corpus/chunks/legacy-role/" + chunk.ID + ".json"
		if err := writeJSONAtomic(path, chunk); err != nil {
			t.Fatal(err)
		}
	}
	ordered, err := corpus.ListDocumentChunks(ctx, document.ID)
	if err != nil || len(ordered) != len(created) {
		t.Fatalf("list chunks=%d err=%v", len(ordered), err)
	}
	for index := range created {
		if ordered[index].ContentHash != created[index].ContentHash || ordered[index].Sequence != index+1 {
			t.Fatalf("legacy order not restored at %d: got=%s want=%s", index, ordered[index].ContentHash, created[index].ContentHash)
		}
	}
}
