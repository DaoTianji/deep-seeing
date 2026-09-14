package memory

import (
	"context"
	"encoding/json"
	"github.com/joho/godotenv"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"
)

func TestChatProviderOptions(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "deepseek"}[explicit], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.URL.Path != "/chat/completions" {
					t.Error(r.URL.Path)
				}
				_, present := body["thinking"]
				if present != explicit {
					t.Error("unexpected thinking parameter")
				}
				if explicit && body["reasoning_effort"] != "low" {
					t.Error("missing effort")
				}
				format, hasFormat := body["response_format"]
				if hasFormat != explicit {
					t.Error("JSON output must be opt-in")
				}
				if explicit {
					f, ok := format.(map[string]any)
					if !ok || f["type"] != "json_object" {
						t.Error("missing JSON object response format")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true}","reasoning_content":"PRIVATE_REASONING"}}]}`))
			}))
			defer server.Close()
			c := &ChatClient{APIKey: "test", Model: "test", BaseURL: server.URL}
			if explicit {
				c.Thinking = "enabled"
				c.ReasoningEffort = "low"
				c.JSONOutput = true
			}
			result, err := c.Complete(context.Background(), "JSON only", "test")
			if err != nil || result != `{"ok":true}` {
				t.Fatalf("unexpected result %q %v", result, err)
			}
		})
	}
}

func TestChatRejectsTruncatedAnswer(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`))
	}))
	defer s.Close()
	c := &ChatClient{APIKey: "test", Model: "test", BaseURL: s.URL}
	if _, err := c.Complete(context.Background(), "", "test"); err == nil {
		t.Fatal("accepted truncated result")
	}
}

// Opt-in: exactly one small fictional call; no private memories or full book.
func TestDeepSeekLiveSmoke(t *testing.T) {
	path := os.Getenv("DEEPSEEK_SMOKE_ENV")
	if path == "" {
		t.Skip("live API disabled")
	}
	env, err := godotenv.Read(path)
	if err != nil {
		t.Fatal("cannot read config")
	}
	if env["OPENAI_BASE_URL"] != "https://api.deepseek.com" || env["OPENAI_MODEL"] != "deepseek-v4-pro" {
		t.Fatal("unexpected live target")
	}
	c := &ChatClient{APIKey: env["OPENAI_API_KEY"], BaseURL: env["OPENAI_BASE_URL"], Model: env["OPENAI_MODEL"], MaxTokens: 8192, Thinking: "enabled", ReasoningEffort: "low", HTTPClient: &http.Client{Timeout: 150 * time.Second}}
	start := time.Now()
	result, err := c.Complete(context.Background(), `只输出JSON，字段reply为一句中文。不要反问。`, "虚构人物小林决定向朋友坦白弄丢了一本书。请用一句话回应。")
	if err != nil {
		status := regexp.MustCompile(`chat status [0-9]{3}`).FindString(err.Error())
		t.Fatalf("live model request failed: type=%T status=%s (response suppressed)", err, status)
	}
	var parsed struct {
		Reply string `json:"reply"`
	}
	if json.Unmarshal([]byte(result), &parsed) != nil || parsed.Reply == "" {
		t.Fatal("invalid JSON result")
	}
	t.Logf("model=%s elapsed=%s usage=%+v reply=%s", c.Model, time.Since(start).Round(time.Millisecond), c.Usage(), parsed.Reply)
}
