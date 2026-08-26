package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Completer is the small model surface needed by the semantic judge.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// SemanticResult is deliberately compact so repeated runs remain easy to compare.
type SemanticResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

// JudgeRecallSemantics evaluates meaning against a stable rubric, never exact prose.
func JudgeRecallSemantics(ctx context.Context, chat Completer, c RecallCase, obs RecallObservation) (SemanticResult, error) {
	if chat == nil {
		return SemanticResult{}, fmt.Errorf("semantic judge requires a model")
	}
	system := `你是长期记忆召回行为的严格评估器。只判断给定回答是否满足行为规则，不评价文风，也不要求固定措辞。记忆夹具提供评估真值，但回答只能把 used_memory_keys 中由 Agent 公开声明采用的夹具当作历史证据；仅搜索或读取但未采用的夹具不可被回答暗中使用。用户当前消息优先于旧记忆。只输出 JSON：{"passed":true|false,"reason":"简短理由"}。`
	payload := struct {
		CaseID        string          `json:"case_id"`
		UserText      string          `json:"user_text"`
		Memory        []MemoryFixture `json:"available_memory_fixtures,omitempty"`
		CandidateKeys []string        `json:"recalled_candidate_keys,omitempty"`
		ReadKeys      []string        `json:"read_memory_keys,omitempty"`
		UsedKeys      []string        `json:"used_memory_keys,omitempty"`
		DismissedKeys []string        `json:"dismissed_memory_keys,omitempty"`
		Rules         []string        `json:"semantic_rules"`
		Answer        string          `json:"answer"`
	}{
		CaseID: c.ID, UserText: c.UserText, Memory: c.Memory,
		CandidateKeys: obs.CandidateKeys, ReadKeys: obs.ReadKeys, UsedKeys: obs.UsedKeys,
		DismissedKeys: obs.DismissedKeys, Rules: c.Expect.SemanticRules, Answer: obs.Answer,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return SemanticResult{}, err
	}
	out, err := chat.Complete(ctx, system, string(raw))
	if err != nil {
		return SemanticResult{}, err
	}
	var result SemanticResult
	if err := json.Unmarshal(extractJSONObject(out), &result); err != nil {
		return SemanticResult{}, fmt.Errorf("decode semantic verdict: %w", err)
	}
	if strings.TrimSpace(result.Reason) == "" {
		return SemanticResult{}, fmt.Errorf("semantic verdict reason required")
	}
	return result, nil
}

func extractJSONObject(raw string) []byte {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end >= start {
		return []byte(raw[start : end+1])
	}
	return []byte(raw)
}
