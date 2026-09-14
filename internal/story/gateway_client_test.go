package story

import (
	"testing"
	"time"
)

func TestReadingGatewayModelAliasesUseSameSettings(t *testing.T) {
	for _, model := range []string{"deepseek-v4-pro", "deepseek-ai/DeepSeek-V4-Pro"} {
		c := NewReadingGatewayClient("fixture-key", "https://example.invalid/v1", model)
		if c.Model != model || c.MaxTokens != 8192 || c.Thinking != "enabled" || c.ReasoningEffort != "low" || !c.JSONOutput || c.HTTPClient.Timeout != 150*time.Second {
			t.Fatalf("settings differ for %s", model)
		}
	}
	c := NewReadingGatewayClient("fixture-key", "https://example.invalid/v1", "other-model")
	if c.MaxTokens != 2500 || c.Thinking != "" || c.ReasoningEffort != "" || c.JSONOutput {
		t.Fatal("unrelated model settings changed")
	}
}
