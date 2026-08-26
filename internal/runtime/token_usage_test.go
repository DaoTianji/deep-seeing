package runtime

import (
	"testing"

	"github.com/cloudwego/eino/components/model"
)

func TestTurnTokenCounterAggregatesModelCalls(t *testing.T) {
	counter := &turnTokenCounter{}
	counter.Add(&model.TokenUsage{
		PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120,
		CompletionTokensDetails: model.CompletionTokensDetails{ReasoningTokens: 7},
	})
	counter.Add(&model.TokenUsage{
		PromptTokens: 40, CompletionTokens: 10, TotalTokens: 50,
		CompletionTokensDetails: model.CompletionTokensDetails{ReasoningTokens: 3},
	})

	got := counter.Snapshot()
	if got.PromptTokens != 140 || got.CompletionTokens != 30 || got.ReasoningTokens != 10 || got.TotalTokens != 170 {
		t.Fatalf("unexpected aggregate: %+v", got)
	}
}

func TestTurnTokenCounterIgnoresNilUsage(t *testing.T) {
	var counter *turnTokenCounter
	counter.Add(nil)
	if got := counter.Snapshot(); got.TotalTokens != 0 {
		t.Fatalf("nil counter should return zero usage: %+v", got)
	}
}
