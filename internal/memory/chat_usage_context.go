package memory

import (
	"context"
	"encoding/json"
)

type chatUsageObserverKey struct{}

// WithChatUsageObserver reports only complete provider-supplied usage, scoped
// to this context. It never includes prompts, content, reasoning or credentials.
// The callback must be safe if callers use the context concurrently.
func WithChatUsageObserver(ctx context.Context, observe func(ChatUsage)) context.Context {
	return context.WithValue(ctx, chatUsageObserverKey{}, observe)
}

func observeChatUsage(ctx context.Context, raw json.RawMessage) {
	observe, _ := ctx.Value(chatUsageObserverKey{}).(func(ChatUsage))
	if observe == nil {
		return
	}
	var wire struct {
		Prompt     *int `json:"prompt_tokens"`
		Completion *int `json:"completion_tokens"`
		Total      *int `json:"total_tokens"`
	}
	if json.Unmarshal(raw, &wire) != nil || wire.Prompt == nil || wire.Completion == nil || wire.Total == nil {
		return
	}
	if *wire.Prompt < 0 || *wire.Completion < 0 || *wire.Total < 0 {
		return
	}
	observe(ChatUsage{PromptTokens: *wire.Prompt, CompletionTokens: *wire.Completion, TotalTokens: *wire.Total})
}
