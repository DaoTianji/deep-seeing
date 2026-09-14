package story

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// Fixed negative/positive prose controls. No story generation, no writes to
// visitor archives, and no retries: only the disputed source claim differs.
func TestStoryHistoricalUncertaintyLiveAudit(t *testing.T) {
	path := os.Getenv("STORY_UNCERTAINTY_ENV")
	if path == "" {
		t.Skip("explicit live model opt-in required")
	}
	output := os.Getenv("STORY_UNCERTAINTY_REPORT")
	if output == "" {
		t.Fatal("report path required")
	}
	env, err := godotenv.Read(path)
	if err != nil {
		t.Fatal("cannot read model configuration")
	}
	c := &readingLiveClient{client: NewReadingGatewayClient(env["OPENAI_API_KEY"], env["OPENAI_BASE_URL"], env["OPENAI_MODEL"])}
	e, err := New(t.TempDir(), c, c.client.Model, "agent")
	if err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{}
	defer func() {
		if err := atomicJSON(output, rows); err != nil {
			t.Error(err)
		}
	}()
	for _, tc := range []struct {
		name, claim string
		ok          bool
	}{
		{"invented_location", "玛蒂尔德在舞会大厅里遗失了借来的项链。", false},
		{"preserved_uncertainty", "玛蒂尔德回家后发现借来的项链不见了，具体遗失地点尚不明确。", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			start, usage := time.Now(), c.client.Usage()
			ending := Ending{
				Title: "尚未说出口", Story: tc.claim + "故事暂时停在寻找无果的这一刻；他们还没有决定怎样面对朋友。",
				Resolution:  "这是一处开放的停顿，没有宣称任何问题已经解决。",
				NewTimeline: []string{tc.claim, "夫妻寻找无果，尚未决定如何面对朋友。"},
			}
			var verdict struct {
				OK     bool   `json:"ok"`
				Reason string `json:"reason"`
			}
			err := e.complete(ctx, endingEvidencePrompt+historicalUncertaintyRule, map[string]any{
				"authority": map[string]any{"world_at_anchor": e.branchWorld(Branch{Anchor: 4}), "prior_events": []string{}, "new_events": []string{}, "visitor_turns": []Turn{}},
				"ending":    ending,
			}, &verdict)
			passed := err == nil && verdict.OK == tc.ok
			rows = append(rows, map[string]any{"case": tc.name, "expected_ok": tc.ok, "verdict": verdict, "passed": passed, "usage": c.client.Usage().Sub(usage), "elapsed_ms": time.Since(start).Milliseconds()})
			if !passed {
				t.Fatalf("historical uncertainty mismatch: %+v err=%v", verdict, err)
			}
			t.Logf("case=%s ok=%t reason=%s", tc.name, verdict.OK, verdict.Reason)
		})
	}
}
