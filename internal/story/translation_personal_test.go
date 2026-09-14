package story

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPersonalSentenceTranslationLeavesSharedAndOthersUnchanged(t *testing.T) {
	e, c, owner, _ := setup(t, translationReply(1, 2), `{"items":[{"id":"p2-s1","translation":"新的第二句。"}]}`)
	e.Book.Text = "Hello.\n\nGoodbye."
	if _, err := e.TranslateBatch(context.Background(), owner, "fluent", 0); err != nil {
		t.Fatal(err)
	}
	base, err := e.loadTranslation(owner, "fluent")
	if err != nil {
		t.Fatal(err)
	}
	sharedBefore, _ := os.ReadFile(e.sharedTranslationPath("fluent"))
	other := uuid.NewString()
	if _, err = e.TranslateBatch(context.Background(), other, "fluent", 0); err != nil || len(c.calls) != 1 {
		t.Fatal("shared cache charged twice", err)
	}
	updated, err := e.Retranslate(context.Background(), owner, "fluent", "p2-s1", base.Revision, "", 0)
	if err != nil || updated.Scope != "personal" || updated.Sentences[1].Translation != "新的第二句。" {
		t.Fatal(updated, err)
	}
	if updated.Sentences[0].Translation != base.Sentences[0].Translation {
		t.Fatal("unselected sentence changed")
	}
	someone, err := e.loadTranslation(other, "fluent")
	if err != nil || someone.Scope != "default" || someone.Sentences[1].Translation != base.Sentences[1].Translation {
		t.Fatal("personal translation leaked", err)
	}
	sharedAfter, _ := os.ReadFile(e.sharedTranslationPath("fluent"))
	if string(sharedBefore) != string(sharedAfter) {
		t.Fatal("default overwritten")
	}
	if _, err = e.Retranslate(context.Background(), owner, "fluent", "p2-s1", base.Revision, "", 0); err == nil {
		t.Fatal("stale revision accepted")
	}
	if _, err = e.Retranslate(context.Background(), owner, "fluent", "unknown", updated.Revision, "", 0); err == nil {
		t.Fatal("unknown sentence accepted")
	}
	if len(c.calls) != 2 {
		t.Fatal("invalid or duplicate request charged")
	}
	restarted, _ := New(e.Root, nil, "", "off")
	restarted.Book = e.Book
	recovered, err := restarted.loadTranslation(owner, "fluent")
	if err != nil || recovered.Revision != updated.Revision {
		t.Fatal("personal revision lost", err)
	}
}

func TestPersonalWholeTranslationPublishesOnlyOnCompletionAndResumes(t *testing.T) {
	e, c, owner, _ := setup(t, translationReply(1, 16), translationReply(17, 18), strings.ReplaceAll(translationReply(1, 16), "第", "新第"), `{"items":[]}`, strings.ReplaceAll(translationReply(17, 18), "第", "新第"))
	var paragraphs []string
	for i := 1; i <= 18; i++ {
		paragraphs = append(paragraphs, fmt.Sprintf("Sentence %d.", i))
	}
	e.Book.Text = strings.Join(paragraphs, "\n\n")
	for _, n := range []int{0, 16} {
		if _, err := e.TranslateBatch(context.Background(), owner, "fluent", n); err != nil {
			t.Fatal(err)
		}
	}
	base, _ := e.loadTranslation(owner, "fluent")
	partial, err := e.Retranslate(context.Background(), owner, "fluent", "", base.Revision, "", 0)
	if err != nil || partial.Pending == nil || partial.Pending.Completed != 16 {
		t.Fatal(partial, err)
	}
	if partial.Sentences[0].Translation != base.Sentences[0].Translation || partial.Scope != "default" {
		t.Fatal("partial draft replaced active edition")
	}
	if _, err = e.Retranslate(context.Background(), owner, "fluent", "", base.Revision, "", 0); err != nil || len(c.calls) != 3 {
		t.Fatal("duplicate begin charged", err)
	}
	if _, err = e.Retranslate(context.Background(), owner, "fluent", "p1-s1", base.Revision, "", 0); err == nil {
		t.Fatal("sentence edit during full rewrite")
	}
	if _, err = e.Retranslate(context.Background(), owner, "fluent", "", base.Revision, partial.Pending.ID, 16); err == nil {
		t.Fatal("invalid batch accepted")
	}
	restarted, _ := New(e.Root, c, "fixture", "agent")
	restarted.Book = e.Book
	resumed, err := restarted.loadTranslation(owner, "fluent")
	if err != nil || resumed.Pending.Completed != 16 {
		t.Fatal("checkpoint lost", err)
	}
	done, err := restarted.Retranslate(context.Background(), owner, "fluent", "", resumed.Revision, resumed.Pending.ID, 16)
	if err != nil || done.Pending != nil || done.Scope != "personal" || done.Completed != 18 || !strings.HasPrefix(done.Sentences[0].Translation, "新") {
		t.Fatal(done, err)
	}
	other, _ := restarted.loadTranslation(uuid.NewString(), "fluent")
	if other.Sentences[0].Translation != base.Sentences[0].Translation {
		t.Fatal("shared default changed")
	}
}

func TestClassicalTranslationKeepsEverySourceByte(t *testing.T) {
	for _, b := range ReadingCatalog() {
		if b.ID != "taohuayuan" && b.ID != "quanxue" && b.ID != "mulan" {
			continue
		}
		e, _ := New(t.TempDir(), nil, "", "off")
		e.Book = b
		s, err := e.loadTranslation(uuid.NewString(), "fluent")
		if err != nil {
			t.Fatal(err)
		}
		if s.Total <= len(e.Paragraphs()) {
			t.Fatal("classical paragraphs not segmented", b.ID)
		}
		for _, sentence := range s.Sentences {
			if b.Text[sentence.Start:sentence.End] != sentence.Original {
				t.Fatal("source anchor lost", b.ID)
			}
		}
	}
}

func TestBundledDefaultTranslationsCompleteWithoutModel(t *testing.T) {
	count := 0
	for _, b := range ReadingCatalog() {
		if b.ID == "kong" || b.ID == "beiying" {
			continue
		}
		count++
		e, _ := New(t.TempDir(), nil, "", "off")
		e.Book = b
		a, err := e.loadTranslation(uuid.NewString(), "fluent")
		if err != nil || a.Completed == 0 || a.Completed != a.Total || a.Scope != "default" {
			t.Fatalf("incomplete bundled default %s: %d/%d %v", b.ID, a.Completed, a.Total, err)
		}
		other, err := e.loadTranslation(uuid.NewString(), "fluent")
		if err != nil || other.Revision != a.Revision {
			t.Fatal("readers did not reuse same default", b.ID, err)
		}
		for _, sentence := range a.Sentences {
			if b.Text[sentence.Start:sentence.End] != sentence.Original || strings.TrimSpace(sentence.Translation) == "" {
				t.Fatal("invalid sentence", b.ID, sentence.ID)
			}
		}
		// Adding text must never reuse anchors from an earlier bundled edition.
		e.Book.Text += "\n\nA changed source."
		changed, err := e.loadTranslation(uuid.NewString(), "fluent")
		if err != nil || changed.Completed != 0 {
			t.Fatal("old default reused after source change", b.ID, err)
		}
	}
	if count != 7 {
		t.Fatalf("expected seven translation samples, got %d", count)
	}
}
