package compaction_test

import (
	"context"
	"strings"
	"testing"

	"deep-seeing/internal/compaction"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/transcript"
)

type fakeComplete struct {
	text string
	err  error
}

func (f fakeComplete) Complete(context.Context, string, string) (string, error) {
	return f.text, f.err
}

func TestSummarizingCompactorWriteBackShape(t *testing.T) {
	c := compaction.NewSummarizingCompactor(compaction.Config{
		MaxTokens:    100000,
		MaxMessages:  4,
		MinTail:      2,
		TriggerRatio: 1.0,
	}, fakeComplete{text: "用户讨论了命名与偏好。"})

	msgs := []transcript.Message{
		transcript.User("1"),
		transcript.Assistant("a1"),
		transcript.User("2"),
		transcript.Assistant("a2"),
		transcript.User("3"),
		transcript.Assistant("a3"),
	}
	out, report, err := c.MaybeCompact(context.Background(), identity.LocalCLI(), "s", msgs, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Applied || !report.WriteBack {
		t.Fatalf("report=%+v", report)
	}
	if out[0].Role != transcript.RoleSummary {
		t.Fatalf("first should be summary: %+v", out[0])
	}
	if !strings.Contains(out[0].Content, "会话摘要") {
		t.Fatalf("missing summary prefix: %s", out[0].Content)
	}
	if len(out) != 3 { // summary + minTail 2
		t.Fatalf("len=%d want 3: %+v", len(out), out)
	}
	if out[1].Content != "3" || out[2].Content != "a3" {
		t.Fatalf("tail wrong: %+v", out)
	}
}

func TestSummarizingCompactorFallsBackOnCompleteError(t *testing.T) {
	c := compaction.NewSummarizingCompactor(compaction.Config{
		MaxTokens:    100000,
		MaxMessages:  4,
		MinTail:      2,
		TriggerRatio: 1.0,
	}, fakeComplete{err: context.DeadlineExceeded})

	msgs := []transcript.Message{
		transcript.User("1"),
		transcript.Assistant("a1"),
		transcript.User("2"),
		transcript.Assistant("a2"),
		transcript.User("3"),
		transcript.Assistant("a3"),
	}
	out, report, err := c.MaybeCompact(context.Background(), identity.LocalCLI(), "s", msgs, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Applied {
		t.Fatal("expected trim fallback")
	}
	if out[0].Role == transcript.RoleSummary {
		t.Fatal("fallback should not produce summary")
	}
}
