package story

import (
	"net/http"
	"strings"
	"time"

	"deep-seeing/internal/memory"
)

// Share settings between the isolated reader and its live acceptance tests.
// Gateways can namespace the same model (deepseek-ai/DeepSeek-V4-Pro); the
// namespace must not silently remove its configured output/thinking allowance.
func NewReadingGatewayClient(apiKey, baseURL, model string) *memory.ChatClient {
	c := &memory.ChatClient{APIKey: apiKey, BaseURL: baseURL, Model: model, MaxTokens: 2500, HTTPClient: &http.Client{Timeout: 150 * time.Second}}
	name := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if strings.HasPrefix(name, "deepseek-v4-") {
		c.MaxTokens = 8192
		c.Thinking = "enabled"
		c.ReasoningEffort = "low"
		// All reader calls expect JSON; prompt examples still define the schema.
		// JSON mode constrains syntax only, never source or semantic correctness.
		c.JSONOutput = true
	}
	return c
}
