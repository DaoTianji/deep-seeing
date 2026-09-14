package story

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type authorModel struct {
	inputs      []string
	reads       int
	badEvidence bool
	reject      bool
	generated   string
}

func (m *authorModel) Complete(_ context.Context, system, input string) (string, error) {
	m.inputs = append(m.inputs, input)
	if strings.Contains(system, "阅读器") {
		m.reads++
		var d struct {
			Chunks []AuthorChunk `json:"chunks"`
		}
		_ = json.Unmarshal([]byte(input), &d)
		cid := d.Chunks[0].ID
		if m.badEvidence {
			cid = "missing:c1"
		}
		b, _ := json.Marshal(map[string]any{"observations": []AuthorObservation{{Kind: "style", Text: "使用具体物象和简短句子。", Period: "仅适用于这份材料", Evidence: []string{cid}, Uncertainty: "不是稳定人格"}}})
		return string(b), nil
	}
	if strings.Contains(system, "综合reading_observations") {
		var d struct {
			Observations []AuthorObservation `json:"reading_observations"`
		}
		_ = json.Unmarshal([]byte(input), &d)
		b, _ := json.Marshal(AuthorAnalysis{Observations: d.Observations, Changes: "根据新增作品修订原有范围", Warnings: []string{}})
		return string(b), nil
	}
	if strings.Contains(system, "独立作者研究审查员") || strings.Contains(system, "核验AI辅助") {
		if m.reject {
			return `{"ok":false}`, nil
		}
		return `{"ok":true}`, nil
	}
	if strings.Contains(system, "写作助手") {
		return `{"title":"窗边","text":"GENERATED_ONLY。我把表放下，等那朵花慢慢开放。","warnings":[]}`, nil
	}
	return "", errors.New("unexpected author prompt")
}
func authorFixture(t *testing.T) (*Engine, *authorModel, string, AuthorProfile) {
	t.Helper()
	m := &authorModel{}
	e, err := New(t.TempDir(), m, "test", "agent")
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	a, err := e.CreateAuthor(owner, "林舟", "虚构测试作者")
	if err != nil {
		t.Fatal(err)
	}
	return e, m, owner, a
}
func addWork(t *testing.T, e *Engine, o string, a AuthorProfile, text string) AuthorProfile {
	t.Helper()
	a, err := e.AddAuthorWork(o, a.ID, AuthorWork{Title: "文章", Text: text, Kind: "original", Private: true}, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestAuthorWorkDatesDedupVersionsAndIsolation(t *testing.T) {
	e, _, o, a := authorFixture(t)
	a = addWork(t, e, o, a, "第一段。\n\n第二段。")
	w := a.Works[0]
	if w.Written != "" || w.Published != "" || w.Revised != "" {
		t.Fatal("upload became authorship date")
	}
	if _, err := e.AddAuthorWork(o, a.ID, AuthorWork{Title: "转载", Text: "第一段。 第二段。", Kind: "reprint"}, a.Revision); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err := e.GetAuthor(uuid.NewString(), a.ID); err == nil {
		t.Fatal("cross visitor access")
	}
	if _, err := e.GetAuthor("../", a.ID); err == nil {
		t.Fatal("path escape")
	}
	a, err := e.AddAuthorWork(o, a.ID, AuthorWork{Title: "修订版", Text: "第一段修订。", Kind: "original", Revises: w.ID, Written: "2018", Revised: "2024"}, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := e.authorPath(o, a.ID)
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("file not private")
	}
	if a.Works[0].Text != w.Text || a.Works[1].Revises != w.ID {
		t.Fatal("revision overwrote original")
	}
	if canonicalAuthorURL("https://EXAMPLE.org/article?utm_source=test#x") != "https://example.org/article" {
		t.Fatal("canonical URL")
	}
}

func TestAuthorDemoIsLabelledGroundedAndVisitorOwned(t *testing.T) {
	e, m, owner, _ := authorFixture(t)
	a, err := e.CreateAuthorDemo(owner)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Aliases, "预生成") || len(a.Works) != 2 || len(a.Analyses) != 1 || len(a.Drafts) != 1 {
		t.Fatal("incomplete or unlabelled demo")
	}
	if len(m.inputs) != 0 {
		t.Fatal("opening demo called model")
	}
	if err = validateAuthorObservations(a.Analyses[0].Observations, authorChunkIndex(a)); err != nil {
		t.Fatal(err)
	}
	if _, err = e.GetAuthor(uuid.NewString(), a.ID); err == nil {
		t.Fatal("demo leaks across visitors")
	}
	b, err := e.CreateAuthorDemo(owner)
	if err != nil || a.ID == b.ID {
		t.Fatal("demo overwrote an existing archive")
	}
	if _, err = e.GetAuthor(owner, a.ID); err != nil {
		t.Fatal("original demo disappeared")
	}
}
func TestAuthorConsentAndUnreadEvidenceGate(t *testing.T) {
	e, m, o, a := authorFixture(t)
	a = addWork(t, e, o, a, "只是测试文本。")
	if _, err := e.AnalyzeAuthor(context.Background(), o, a.ID, AuthorOperation{Revision: a.Revision}, nil); err == nil {
		t.Fatal("no consent accepted")
	}
	if len(m.inputs) > 0 {
		t.Fatal("model called before consent")
	}
	m.badEvidence = true
	if _, err := e.AnalyzeAuthor(context.Background(), o, a.ID, AuthorOperation{Revision: a.Revision, Consent: true}, nil); err == nil {
		t.Fatal("unread evidence accepted")
	}
	stored, _ := e.GetAuthor(o, a.ID)
	if len(stored.Analyses) != 0 {
		t.Fatal("invalid analysis persisted")
	}
}

func TestAuthorReferenceNormalizationOnlyMatchesReadIDs(t *testing.T) {
	index := map[string]AuthorChunk{"w1:c1": {ID: "w1:c1", Text: "一段真正存在的原文。"}}
	items := []AuthorObservation{{Kind: "style", Text: "有限特征", Evidence: []string{"id:w1:c1"}}}
	normalizeAuthorReferences(items, index)
	if err := validateAuthorObservations(items, index); err != nil {
		t.Fatal(err)
	}
	items[0].Evidence = []string{"id:w2:c1"}
	normalizeAuthorReferences(items, index)
	if err := validateAuthorObservations(items, index); err == nil {
		t.Fatal("guessed unread source")
	}
	items[0].Evidence = []string{"w1:c1:真正存在的原文"}
	normalizeAuthorReferences(items, index)
	if err := validateAuthorObservations(items, index); err != nil {
		t.Fatal(err)
	}
	items[0].Evidence = []string{"w1:c1:伪造的引语"}
	normalizeAuthorReferences(items, index)
	if err := validateAuthorObservations(items, index); err == nil {
		t.Fatal("fabricated quote normalized")
	}
}
func TestAuthorReadsWholeWorkAndReusesCheckpoints(t *testing.T) {
	e, m, o, a := authorFixture(t)
	text := strings.Repeat("完整文章中的细节。", 2500)
	a = addWork(t, e, o, a, text)
	chunks := authorChunks(a.Works[0])
	if len(chunks) < 5 {
		t.Fatal("fixture not long")
	}
	joined := ""
	for _, c := range chunks {
		joined += c.Text
	}
	if joined != text {
		t.Fatal("chunk lost content")
	}
	a, err := e.AnalyzeAuthor(context.Background(), o, a.ID, AuthorOperation{Revision: a.Revision, Consent: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	readCalls := m.reads
	if readCalls != (len(chunks)+3)/4 || len(a.Analyses) != 1 {
		t.Fatal("did not read all batches")
	}
	a, err = e.AnalyzeAuthor(context.Background(), o, a.ID, AuthorOperation{Revision: a.Revision, Consent: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.reads != readCalls || len(a.Analyses) != 2 || a.Analyses[0].Version != 1 {
		t.Fatal("checkpoint/version history")
	}
	files, _ := filepath.Glob(filepath.Join(e.Root, "authors", o, a.ID, "reads", a.Works[0].ID, "*.json"))
	if len(files) != readCalls {
		t.Fatal("missing checkpoints")
	}
}
func TestAuthorCriticRejectsFormalAnalysis(t *testing.T) {
	e, m, o, a := authorFixture(t)
	a = addWork(t, e, o, a, "一个具体细节。")
	m.reject = true
	_, err := e.AnalyzeAuthor(context.Background(), o, a.ID, AuthorOperation{Revision: a.Revision, Consent: true}, nil)
	if err == nil {
		t.Fatal("critic bypassed")
	}
	a, _ = e.GetAuthor(o, a.ID)
	if len(a.Analyses) != 0 {
		t.Fatal("rejected claims persisted")
	}
}
func TestAuthorGeneratedDraftCannotBecomeAnalysisEvidence(t *testing.T) {
	e, m, o, a := authorFixture(t)
	a = addWork(t, e, o, a, "用具体物象表达停留。")
	a, err := e.AnalyzeAuthor(context.Background(), o, a.ID, AuthorOperation{Revision: a.Revision, Consent: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	in := AuthorWritingInput{AuthorOperation: AuthorOperation{Revision: a.Revision, Consent: true}, ObservationIDs: []string{a.Analyses[0].Observations[0].ID}, Brief: "虚构短文", Facts: "完全虚构"}
	a, err = e.WriteWithAuthor(context.Background(), o, a.ID, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Drafts) != 1 || len(a.Works) != 1 {
		t.Fatal("draft polluted works")
	}
	start := len(m.inputs)
	a, err = e.AnalyzeAuthor(context.Background(), o, a.ID, AuthorOperation{Revision: a.Revision, Consent: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range m.inputs[start:] {
		if strings.Contains(input, "GENERATED_ONLY") {
			t.Fatal("generated text became evidence")
		}
	}
	draft := a.Drafts[0]
	if _, err = e.AddAuthorWork(o, a.ID, AuthorWork{Title: "不能回流的生成稿", Text: draft.Text, Kind: "original"}, a.Revision); err == nil {
		t.Fatal("generated draft reimported as source")
	}
	a, err = e.EditAuthorDraft(o, a.ID, draft.ID, "我的修订", a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if a.Drafts[0].Text != draft.Text || a.Drafts[0].Edited != "我的修订" {
		t.Fatal("human edit lost original")
	}
	if _, err = e.EditAuthorDraft(o, a.ID, draft.ID, "过期修改", a.Revision-1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale write accepted")
	}
}
