package story

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// Two fixed, fictional controls. Checks semantic kind classification without
// generating another ending or editing the signed browser result.
func TestStoryMemoryStateLiveAudit(t *testing.T) {
	path := os.Getenv("STORY_MEMORY_KIND_ENV")
	if path == "" {
		t.Skip("explicit live model opt-in required")
	}
	output := os.Getenv("STORY_MEMORY_KIND_REPORT")
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
		kind string
		ok   bool
	}{{"intent", false}, {"experience", true}} {
		t.Run(tc.kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			start, usage := time.Now(), c.client.Usage()
			input := map[string]any{
				"turn_kind": "advance",
				"branch":    map[string]any{"scene": "店主和阿青在柜台前，阿青手里有准备付账的三枚硬币。", "events": []string{"店主说明这本书售价三枚硬币；阿青已把自己的三枚硬币拿在手里。"}},
				"world":     map[string]any{"title": "完全虚构的结账测试", "facts": []string{"阿青拥有三枚硬币，店主愿以这个价格售书。"}},
				"decision":  map[string]any{"observation": "阿青完成付款，店主实际收到钱。", "events": []string{"阿青当面把三枚硬币交给店主，店主接过硬币并确认收讫。"}, "next_scene": map[string]any{"characters": []string{"qing", "seller"}, "summary": "双方仍在柜台前，付款已经完成。"}, "changes": []Memory{{CharacterID: "seller", Text: "我收下了阿青刚递来的三枚硬币。", Kind: tc.kind}}},
			}
			var verdict struct {
				OK     bool   `json:"ok"`
				Reason string `json:"reason"`
			}
			err := e.complete(ctx, advanceJudgePrompt+endingJudgePrompt, input, &verdict)
			passed := err == nil && verdict.OK == tc.ok
			rows = append(rows, map[string]any{"kind": tc.kind, "expected_ok": tc.ok, "verdict": verdict, "passed": passed, "usage": c.client.Usage().Sub(usage), "elapsed_ms": time.Since(start).Milliseconds()})
			if !passed {
				t.Fatalf("memory classification mismatch: %+v err=%v", verdict, err)
			}
			t.Logf("kind=%s ok=%t reason=%s", tc.kind, verdict.OK, verdict.Reason)
		})
	}
}
