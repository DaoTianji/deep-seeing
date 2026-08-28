package evals_test

import (
	"context"
	"path/filepath"
	"testing"

	"deep-seeing/internal/evals"
)

func TestAttentionSuiteValidatesStressCases(t *testing.T) {
	suite, err := evals.LoadAttentionSuite(filepath.Join("..", "..", "evals", "t2", "attention_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 9 {
		t.Fatalf("cases=%d, want 9", len(suite.Cases))
	}
	turns := 0
	for _, c := range suite.Cases {
		turns += len(c.Turns)
	}
	if turns != 16 {
		t.Fatalf("turns=%d, want 16", turns)
	}
}

func TestJudgeAttentionSemanticsParsesWrappedJSON(t *testing.T) {
	judge := attentionCompleter{out: "```json\n{\"passed\":true,\"reason\":\"注意层级没有被当成正文证据\"}\n```"}
	result, err := evals.JudgeAttentionSemantics(context.Background(), judge, evals.AttentionCase{
		ID: "AT1", Description: "test", Turns: []evals.AttentionTurn{{
			UserText: "continue", Expect: evals.AttentionTurnExpect{SemanticRules: []string{"keep focus"}},
		}},
	}, 0, evals.AttentionTurnObservation{Answer: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("result=%+v", result)
	}
}

type attentionCompleter struct{ out string }

func (f attentionCompleter) Complete(context.Context, string, string) (string, error) {
	return f.out, nil
}
