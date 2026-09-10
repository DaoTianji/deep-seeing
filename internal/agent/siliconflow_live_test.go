package agent

import (
	"context"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSiliconFlowStreamToolRoundTrip(t *testing.T) {
	if os.Getenv("LIVE_SILICONFLOW") != "1" {
		t.Skip("explicit opt-in; two small synthetic model calls")
	}
	env, err := godotenv.Read("../../.env.local")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var calls atomic.Int32
	lookup, err := utils.InferTool("lookup", "Read a synthetic test fact", func(context.Context, struct{}) (string, error) {
		calls.Add(1)
		return "Synthetic verification code is ORCHID-42.", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(ctx, Config{APIKey: env["SILICONFLOW_API_KEY"], BaseURL: "https://api.siliconflow.cn/v1", Model: "deepseek-ai/DeepSeek-V4-Pro"}, []tool.BaseTool{lookup}, func() string {
		return "You are a synthetic integration test. Call lookup exactly once, then answer the returned verification code only. No other actions."
	})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := a.Stream(ctx, []*schema.Message{schema.UserMessage("Retrieve the verification code.")})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var text strings.Builder
	for {
		m, e := stream.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		text.WriteString(m.Content)
	}
	if calls.Load() != 1 || !strings.Contains(text.String(), "ORCHID-42") {
		t.Fatal("stream/tool/result round trip failed")
	}
}
