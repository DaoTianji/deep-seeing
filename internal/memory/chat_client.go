package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatClient is a minimal OpenAI-compatible chat caller shared by side-query and extractor.
type ChatClient struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
	MaxTokens  int
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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
