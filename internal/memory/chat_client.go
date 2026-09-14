package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ChatClient is a minimal OpenAI-compatible chat caller shared by side-query and extractor.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u ChatUsage) Sub(before ChatUsage) ChatUsage {
	return ChatUsage{PromptTokens: u.PromptTokens - before.PromptTokens, CompletionTokens: u.CompletionTokens - before.CompletionTokens, TotalTokens: u.TotalTokens - before.TotalTokens}
}

type ChatClient struct {
	mu         sync.Mutex
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
	MaxTokens  int
	// Optional provider parameters; omitted for existing callers.
	Thinking        string
	ReasoningEffort string
	JSONOutput      bool
	usage           ChatUsage
}

func (c *ChatClient) Usage() ChatUsage {
	if c == nil {
		return ChatUsage{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usage
}

func (c *ChatClient) Complete(ctx context.Context, system, user string) (string, error) {
	if c == nil || strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(c.Model) == "" {
		return "", fmt.Errorf("chat client incomplete")
	}
	maxTok := c.MaxTokens
	if maxTok <= 0 {
		maxTok = 512
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	body := map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"max_tokens": maxTok,
	}
	if c.Thinking != "" {
		body["thinking"] = map[string]string{"type": c.Thinking}
	}
	if c.ReasoningEffort != "" {
		body["reasoning_effort"] = c.ReasoningEffort
	}
	if c.JSONOutput {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat status %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}
	var parsed struct {
		Usage   ChatUsage `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	c.mu.Lock()
	c.usage.PromptTokens += parsed.Usage.PromptTokens
	c.usage.CompletionTokens += parsed.Usage.CompletionTokens
	c.usage.TotalTokens += parsed.Usage.TotalTokens
	c.mu.Unlock()
	var receipt struct {
		Usage json.RawMessage `json:"usage"`
	}
	_ = json.Unmarshal(respBody, &receipt)
	observeChatUsage(ctx, receipt.Usage)
	if len(parsed.Choices) == 0 {
		return "", nil
	}
	if parsed.Choices[0].FinishReason == "length" {
		return "", fmt.Errorf("model output exceeded token budget; response not committed")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
