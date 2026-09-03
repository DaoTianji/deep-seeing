package theater

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type bookTestCompleter struct {
	chapterCalls int
	failCall     int
	failed       bool
	seen         map[string]int
}

func (f *bookTestCompleter) Complete(_ context.Context, system, input string) (string, error) {
	if strings.Contains(system, "按顺序真实阅读") {
		f.chapterCalls++
		if f.failCall == f.chapterCalls && !f.failed {
			f.failed = true
			return "", errors.New("synthetic interruption")
		}
		var envelope struct {
			Chunks []RoleChunk `json:"chunks"`
		}
		if err := json.Unmarshal([]byte(input), &envelope); err != nil || len(envelope.Chunks) == 0 {
			return "", errors.New("chapter text missing")
		}
		if f.seen == nil {
			f.seen = map[string]int{}
		}
		for _, chunk := range envelope.Chunks {
			if chunk.Content == "" {
				return "", errors.New("chunk body missing")
			}
			f.seen[chunk.ID]++
		}
		id := envelope.Chunks[0].ID
		out := chapterReadingOutput{
			Summary:           "本章摘要",
			Observations:      []PassageObservation{{Kind: "explicit_fact", Statement: "本章发生了一件可定位的事", ChunkIDs: []string{id}, Explicit: true, Confidence: "high"}},
			Perspectives:      []CharacterPerspectiveFrame{{Character: "林舟", Timepoint: "本章", Knows: []string{"眼前事实"}, Unknowns: []string{"后续结果"}, ChunkIDs: []string{id}}},
			AuthorExpressions: []AuthorExpressionFrame{{Scope: "passage", Topic: "方法", Statement: "本段使用对照", ChunkIDs: []string{id}}},
		}
		raw, _ := json.Marshal(out)
		return string(raw), nil
	}
	return `{"map_summary":"全书按章节推进并发生修正","synthesis":"首尾信息已由逐章回执串联","reading_experience":{"summary":"读完后保留了理解与疑问","resonances":["对修正的重视"],"objections":["不能过度概括"],"questions":["后续如何验证"]}}`, nil
}

func makeBookFixture(t *testing.T) (*Store, *CorpusStore, RoleDefinition, RoleDocument, []RoleChunk) {
	t.Helper()
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "林舟", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	if err != nil {
		t.Fatal(err)
	}
	document, chunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "book-source", Title: "雾港来信", Audience: SourceActor, Tier: SourcePrimary, Text: []byte("# 第一章\n\n" + strings.Repeat("甲", 5200) + "\n\n# 第二章\n\n" + strings.Repeat("乙", 5200) + "\n\n# 第三章\n\n" + strings.Repeat("丙", 5200))})
	if err != nil || len(chunks) < 3 {
		t.Fatalf("ingest book: %v chunks=%d", err, len(chunks))
	}
	return store, corpus, role, document, chunks
}

func TestBookReaderMarksOnlyActuallyProcessedChunksRead(t *testing.T) {
	store, corpus, role, document, chunks := makeBookFixture(t)
	defer corpus.Close()
	chat := &bookTestCompleter{}
	run, err := (&BookReader{Store: store, Corpus: corpus, Chat: chat, Model: "fixture", MaxBatchRunes: 4500}).ReadDocument(context.Background(), role, document.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != ReadingCompleted || run.ReadingExperienceID == "" || run.Synthesis == "" {
		t.Fatalf("incomplete run: %#v", run)
	}
	if len(run.ReadChunkIDs) != len(chunks) || len(run.ReceiptIDs) != len(run.Chapters) {
		t.Fatalf("read=%d chunks=%d receipts=%d chapters=%d", len(run.ReadChunkIDs), len(chunks), len(run.ReceiptIDs), len(run.Chapters))
	}
	for _, chunk := range chunks {
		if chat.seen[chunk.ID] != 1 {
			t.Fatalf("chunk %s model receive count=%d", chunk.ID, chat.seen[chunk.ID])
		}
	}
	receipts, err := store.ListReadingReceipts(context.Background(), run.ID)
	if err != nil || len(receipts) != len(run.Chapters) {
		t.Fatalf("receipts: %v %#v", err, receipts)
	}
	for _, receipt := range receipts {
		if !receipt.Succeeded || receipt.InputHash == "" || len(receipt.ChunkIDs) == 0 {
			t.Fatalf("invalid receipt: %#v", receipt)
		}
	}
}

func TestBookReaderResumesAfterChapterInterruptionWithoutRereading(t *testing.T) {
	store, corpus, role, document, _ := makeBookFixture(t)
	defer corpus.Close()
	chat := &bookTestCompleter{failCall: 2}
	reader := &BookReader{Store: store, Corpus: corpus, Chat: chat, Model: "fixture", MaxBatchRunes: 4500}
	failed, err := reader.ReadDocument(context.Background(), role, document.ID)
	if err == nil || failed.Status != ReadingFailed || failed.NextChapter != 1 || len(failed.ReceiptIDs) != 1 {
		t.Fatalf("expected resumable failure: status=%s next=%d receipts=%d err=%v", failed.Status, failed.NextChapter, len(failed.ReceiptIDs), err)
	}
	firstChunk := failed.Chapters[0].ChunkIDs[0]
	completed, err := reader.ReadDocument(context.Background(), role, document.ID)
	if err != nil || completed.Status != ReadingCompleted {
		t.Fatalf("resume failed: status=%s err=%v", completed.Status, err)
	}
	if chat.seen[firstChunk] != 1 {
		t.Fatalf("completed chapter was reread %d times", chat.seen[firstChunk])
	}
}

func TestReadingOutputRejectsUnseenEvidence(t *testing.T) {
	chapter := BookChapter{ID: "chapter_001", ChunkIDs: []string{"known"}}
	out := chapterReadingOutput{Summary: "摘要", Observations: []PassageObservation{{Statement: "越界", ChunkIDs: []string{"unseen"}}}}
	if err := validateReadingOutput(chapter, &out); err == nil {
		t.Fatal("unseen chunk was accepted as reading evidence")
	}
}

func TestBuildBookChaptersBatchesUntitledPDFPages(t *testing.T) {
	chunks := []RoleChunk{
		{ID: "p1", Page: 1, Content: strings.Repeat("甲", 100)},
		{ID: "p2", Page: 2, Content: strings.Repeat("乙", 100)},
		{ID: "p3", Page: 3, Content: strings.Repeat("丙", 100)},
	}
	chapters := buildBookChapters(chunks, 250)
	if len(chapters) != 2 {
		t.Fatalf("untitled pages produced %d reading units, want 2", len(chapters))
	}
	if len(chapters[0].ChunkIDs) != 2 || chapters[0].PageFrom != 1 || chapters[0].PageTo != 2 {
		t.Fatalf("first reading unit was not packed across pages: %#v", chapters[0])
	}
}

func TestBuildBookChaptersPreservesRealHeadingBoundaries(t *testing.T) {
	chunks := []RoleChunk{
		{ID: "a", Section: "第一章", Page: 1, Content: "甲"},
		{ID: "b", Section: "第二章", Page: 2, Content: "乙"},
	}
	chapters := buildBookChapters(chunks, 1000)
	if len(chapters) != 2 || chapters[0].Title != "第一章" || chapters[1].Title != "第二章" {
		t.Fatalf("real headings were merged: %#v", chapters)
	}
}

func TestEvidenceConfidenceAcceptsModelLabelsAndScores(t *testing.T) {
	for raw, want := range map[string]EvidenceConfidence{`"high"`: "high", `0.72`: "medium", `35`: "low"} {
		var got EvidenceConfidence
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		if got != want {
			t.Fatalf("decode %s=%s want %s", raw, got, want)
		}
	}
}

func TestFlexibleStringListAcceptsScalarAndArray(t *testing.T) {
	for raw, want := range map[string]int{`"one"`: 1, `["one","two"]`: 2} {
		var got FlexibleStringList
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		if len(got) != want {
			t.Fatalf("decode %s length=%d want %d", raw, len(got), want)
		}
	}
}

func TestCharacterPerspectiveAcceptsScalarStateLists(t *testing.T) {
	var frame CharacterPerspectiveFrame
	err := json.Unmarshal([]byte(`{"character":"阿德勒","knows":"组织已经决裂","unknowns":"后续评价","beliefs":"合作仍重要","wants":"澄清立场","feelings":"谨慎","chunk_ids":["known"]}`), &frame)
	if err != nil || len(frame.Knows) != 1 || len(frame.Unknowns) != 1 || len(frame.Beliefs) != 1 || len(frame.Wants) != 1 || len(frame.Feelings) != 1 {
		t.Fatalf("scalar perspective lists were not normalized: %#v err=%v", frame, err)
	}
}

func TestFlexibleTextAcceptsProseAndStructuredMap(t *testing.T) {
	for raw := range map[string]bool{`"plain"`: true, `{"opening":"error","ending":"unknown"}`: true} {
		var got FlexibleText
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		if strings.TrimSpace(string(got)) == "" {
			t.Fatalf("decode %s was empty", raw)
		}
	}
}
