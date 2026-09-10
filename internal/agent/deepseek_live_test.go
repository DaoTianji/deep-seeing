package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// Explicit opt-in: at most three small, synthetic requests; no Episode or STM access.
func TestDeepSeekOfficialStreamToolRoundTrip(t *testing.T) {
	if os.Getenv("LIVE_DEEPSEEK") != "1" {
		t.Skip("billable official provider test; explicit opt-in only")
	}
	env, err := godotenv.Read("../../.env.local")
	if err != nil || env["DEEPSEEK_API_KEY"] == "" {
		t.Fatal("missing official key in ignored .env.local")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var requests, lookups atomic.Int32
	lookup, err := utils.InferTool("lookup", "Read a synthetic verification fact", func(context.Context, struct{}) (string, error) {
		if lookups.Add(1) != 1 {
			return "", fmt.Errorf("test lookup already called")
		}
		return "The synthetic verification code is ORCHID-42.", nil
	})
	if err != nil {
		t.Fatal("test tool initialization failed")
	}
	a, err := New(ctx, Config{APIKey: env["DEEPSEEK_API_KEY"], BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-v4-pro"}, []tool.BaseTool{lookup}, func() string {
		return "This is an isolated test. Call lookup exactly once, then reply only with its verification code. Do not invent the code."
	})
	if err != nil {
		t.Fatal("agent initialization failed")
	}
	usage := map[int32]struct{ Input, Output int }{}
	limit := openai.WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, body []byte) ([]byte, error) {
		if requests.Add(1) > 3 || len(body) > 16384 {
			return nil, fmt.Errorf("synthetic test request budget exceeded")
		}
		var req struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			Thinking  struct {
				Type string `json:"type"`
			} `json:"thinking"`
		}
		if json.Unmarshal(body, &req) != nil || req.Model != "deepseek-v4-pro" || req.MaxTokens != 2048 || req.Thinking.Type != "enabled" {
			return nil, fmt.Errorf("official request contract mismatch")
		}
		return body, nil
	})
	meter := openai.WithResponseChunkMessageModifier(func(_ context.Context, msg *schema.Message, body []byte, _ bool) (*schema.Message, error) {
		var event struct {
			Usage struct {
				Input  int `json:"prompt_tokens"`
				Output int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &event) == nil {
			u := usage[requests.Load()]
			u.Input = max(u.Input, event.Usage.Input)
			u.Output = max(u.Output, event.Usage.Output)
			usage[requests.Load()] = u
		}
		return msg, nil
	})
	stream, err := a.Stream(ctx, []*schema.Message{schema.UserMessage("Retrieve the verification code.")}, react.WithChatModelOptions(model.WithMaxTokens(2048), limit, meter))
	if err != nil {
		t.Fatalf("stream failed (%T); response body withheld", err)
	}
	defer stream.Close()
	var reply strings.Builder
	for {
		m, e := stream.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatalf("stream receive failed (%T); response body withheld", e)
		}
		reply.WriteString(m.Content)
	}
	if lookups.Load() != 1 || !strings.Contains(reply.String(), "ORCHID-42") {
		t.Fatal("tool/result assertion failed")
	}
	var input, output int
	for _, u := range usage {
		input += u.Input
		output += u.Output
	}
	if input == 0 && output == 0 {
		t.Logf("official stream/tool PASS: requests=%d lookups=%d usage=unavailable (not free); conservative_request_reservation_cny=%.6f", requests.Load(), lookups.Load(), float64(requests.Load())*(16384*9+2048*27)/1e6)
	} else {
		t.Logf("official stream/tool PASS: requests=%d lookups=%d input_tokens=%d output_tokens=%d conservative_peak_cny=%.6f", requests.Load(), lookups.Load(), input, output, float64(input)*9/1e6+float64(output)*27/1e6)
	}
}
