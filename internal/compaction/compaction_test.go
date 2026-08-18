package compaction_test

import (
	"context"
	"testing"

	"deep-seeing/internal/compaction"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/transcript"
)

func TestThresholdCompactorTrims(t *testing.T) {
	c := compaction.NewThresholdCompactor(100000, 4, 2)
	msgs := []transcript.Message{
		transcript.System("sys"),
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
		t.Fatal("expected compaction")
	}
	if out[0].Role != transcript.RoleSystem {
		t.Fatalf("system prefix lost: %+v", out)
	}
	if len(out) > 5 { // system + up to 4
		t.Fatalf("too many messages: %d", len(out))
	}
}
