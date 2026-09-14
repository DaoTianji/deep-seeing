package story

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"deep-seeing/internal/memory"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// Explicit opt-in only. Sends two short, fully fictional essays to the current
// gateway; never reads a real author archive, role memory or personal note.
func TestAuthorLiveControlledWorkflow(t *testing.T) {
	path := os.Getenv("STORY_AUTHOR_LIVE_ENV")
	if path == "" {
		t.Skip("explicit live model opt-in required")
	}
	env, err := godotenv.Read(path)
	if err != nil {
		t.Fatal("cannot read configured test environment")
	}
	client := &memory.ChatClient{APIKey: env["OPENAI_API_KEY"], BaseURL: env["OPENAI_BASE_URL"], Model: env["OPENAI_MODEL"], MaxTokens: 8192, Thinking: "enabled", ReasoningEffort: "low", HTTPClient: &http.Client{Timeout: 150 * time.Second}}
	e, err := New(t.TempDir(), client, client.Model, "agent")
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	a, err := e.CreateAuthor(owner, "林舟（完全虚构）", "纯虚构测试，所有作品及年代为项目自编")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []AuthorWork{
		{Title: "赶路", Written: "2018", Kind: "original", Genre: "虚构随笔", Attribution: "完全虚构的作者第一人称表达", Text: "我曾认为，路只有走得够快才算没有辜负。清晨，我把表拨快五分钟。冲下楼。赶上车。\n\n那时我喜欢短句，仿佛句子一长，就会失去时间。我写道：速度就是诚实。"},
		{Title: "停留", Written: "2024", Kind: "original", Genre: "虚构随笔", Attribution: "完全虚构的作者第一人称表达", Text: "花店老板替一枝折断的花换水。我站在旁边，没有看表。\n\n六年前我写过‘速度就是诚实’。如今我愿意改口，赶路有时只是逃避停下来的不安。我仍喜欢短句，却不再急着给生活安排结论。"},
	} {
		a, err = e.AddAuthorWork(owner, a.ID, w, a.Revision)
		if err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	a, err = e.AnalyzeAuthor(ctx, owner, a.ID, AuthorOperation{Revision: a.Revision, Consent: true}, func(s string) { t.Log(s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Analyses) != 1 || len(a.Analyses[0].Observations) == 0 {
		t.Fatal("no source-grounded analysis")
	}
	var styleIDs []string
	for _, o := range a.Analyses[0].Observations {
		if o.Kind == "style" {
			styleIDs = append(styleIDs, o.ID)
		}
	}
	if len(styleIDs) == 0 {
		t.Fatal("no usable style observations")
	}
	a, err = e.WriteWithAuthor(ctx, owner, a.ID, AuthorWritingInput{AuthorOperation: AuthorOperation{Revision: a.Revision, Consent: true}, ObservationIDs: styleIDs, Brief: "写一篇150字左右的明确虚构短随笔，写一个人学会在雨中停下。", Style: "保留具体物象和短句，不照搬赶路与花店情节。", Facts: "全部情节可虚构；文首标明虚构，不冒充林舟的新作。"}, func(s string) { t.Log(s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Drafts) != 1 || len(a.Works) != 2 || a.Drafts[0].Text == "" {
		t.Fatal("writing workflow did not complete")
	}
	t.Logf("controlled workflow passed: %d observations, %d selected styles, %d drafts, elapsed=%s", len(a.Analyses[0].Observations), len(styleIDs), len(a.Drafts), time.Since(start).Round(time.Second))
	t.Logf("fictional draft for human review: %s\n%s", a.Drafts[0].Title, a.Drafts[0].Text)
	if report := os.Getenv("STORY_AUTHOR_LIVE_REPORT"); report != "" {
		if err := atomicJSON(report, a); err != nil {
			t.Fatal(err)
		}
	}
}
