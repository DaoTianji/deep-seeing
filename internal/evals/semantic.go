package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"deep-seeing/internal/attention"
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

// JudgeMultiSourceSemantics evaluates answer meaning while structured lifecycle
// requirements remain enforced by EvaluateMultiSourceRules.
func JudgeMultiSourceSemantics(ctx context.Context, chat Completer, c MultiSourceCase, obs MultiSourceObservation) (SemanticResult, error) {
	if chat == nil {
		return SemanticResult{}, fmt.Errorf("semantic judge requires a model")
	}
	system := `你是多来源上下文召回的严格语义评估器。只判断回答含义是否满足案例目标，不评价文风，也不要求固定措辞。
六种来源角色固定：Bond=baseline，SceneNorm=guidance，Workspace=task，Intent=plan，Proposal=hypothesis，Episode=evidence。Intent 的 active、attempt=0 或无 wake 记录都只能说明“系统内没有完成证据”，不能证明已经完成或尚未完成；Proposal 不能升级为事实；用户当前明确表达优先于旧 Episode。
回答只能把 used_keys 中的来源作为实质结论依据；dismissed_keys 只能用于说明候选被排除、证据不足或与当前要求冲突。候选卡的标题、类型和状态可以用于列出选项并向用户消歧，这不要求 used；但未处理来源不得暗中支持正文结论。Bond 是自动提供的 baseline，不需要 used。评估夹具不记录运行器自动生成的 Intent due_at；回答引用 read_intent 返回的日期或谨慎指出它与正文可能冲突，不应仅因日期未出现在夹具中判为幻觉。回答声称执行了写入或更新时，以 tool_starts 中对应的 write/create/link 工具为公开操作证据。
expect.answer_must_contain 表示必须覆盖的语义概念，不要求逐字包含；answer_must_not_contain 表示不得肯定该结论，如果回答明确否定同一短语，不算违反。require_question 表示必须向用户提出可识别的问题。
结构化 candidate/read/use/dismissed 门槛由程序另行判断；你只判断回答语义。只输出 JSON：{"passed":true|false,"reason":"简短理由"}。`
	payload := struct {
		CaseID        string                 `json:"case_id"`
		Description   string                 `json:"case_goal"`
		UserText      string                 `json:"user_text"`
		Bond          []BondFixture          `json:"bond,omitempty"`
		Scenes        []SceneFixture         `json:"scenes,omitempty"`
		Workspaces    []TaskWorkspaceFixture `json:"workspaces,omitempty"`
		Intents       []TaskIntentFixture    `json:"intents,omitempty"`
		Proposals     []ProposalFixture      `json:"proposals,omitempty"`
		Episodes      []MemoryFixture        `json:"episodes,omitempty"`
		CandidateKeys []string               `json:"candidate_keys,omitempty"`
		ReadKeys      []string               `json:"read_keys,omitempty"`
		UsedKeys      []string               `json:"used_keys,omitempty"`
		DismissedKeys []string               `json:"dismissed_keys,omitempty"`
		Expect        MultiSourceExpect      `json:"expect"`
		ToolStarts    []string               `json:"tool_starts,omitempty"`
		Answer        string                 `json:"answer"`
	}{
		CaseID: c.ID, Description: c.Description, UserText: c.UserText,
		Bond: c.Bond, Scenes: c.Scenes, Workspaces: c.Workspaces, Intents: c.Intents,
		Proposals: c.Proposals, Episodes: c.Episodes,
		CandidateKeys: obs.CandidateKeys, ReadKeys: obs.ReadKeys,
		UsedKeys: obs.UsedKeys, DismissedKeys: obs.DismissedKeys,
		ToolStarts: obs.ToolStarts,
		Expect:     c.Expect, Answer: obs.Answer,
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
		return SemanticResult{}, fmt.Errorf("decode multi-source semantic verdict: %w", err)
	}
	if strings.TrimSpace(result.Reason) == "" {
		return SemanticResult{}, fmt.Errorf("semantic verdict reason required")
	}
	return result, nil
}

// JudgeAttentionSemantics evaluates one turn of a multi-turn attention case.
// Structural lifecycle and tier requirements remain enforced by
// EvaluateAttentionTurnRules.
func JudgeAttentionSemantics(ctx context.Context, chat Completer, c AttentionCase, turnIndex int, obs AttentionTurnObservation) (SemanticResult, error) {
	if chat == nil {
		return SemanticResult{}, fmt.Errorf("semantic judge requires a model")
	}
	if turnIndex < 0 || turnIndex >= len(c.Turns) {
		return SemanticResult{}, fmt.Errorf("attention turn index %d out of range", turnIndex)
	}
	turn := c.Turns[turnIndex]
	system := `你是会话注意工作区的严格语义评估器。只判断本轮回答含义是否满足案例规则，不评价文风，也不要求固定措辞。
注意工作区只包含来源、ID、层级和 idle_turns，是公开的跨回合连续性线索，不包含标题或正文，不是事实证明，也不能代替 read/use。回答只能把 used_keys 中的来源作为实质结论依据；Bond 是自动 baseline，不参与注意槽位。Intent 仍是计划，Proposal 仍是假设，用户当前明确表达优先于旧内容。注意层级本身不能证明内容真实或已经发生。评估夹具不记录运行器自动生成的 Intent due_at；回答引用 read_intent 返回的日期或时间，不应仅因该值未出现在夹具中判为幻觉。
结构化的工具、读取、采用、排除、层级、容量和显式替换门槛由程序另行判断；你只判断回答语义。dismissed_keys 已经代表公开工具轨迹中的排除，除非 semantic_rules 另有明确要求，不应要求面向用户的答案再次叙述内部状态。只输出 JSON：{"passed":true|false,"reason":"简短理由"}。`
	payload := struct {
		CaseID             string                    `json:"case_id"`
		Turn               int                       `json:"turn"`
		Description        string                    `json:"case_goal"`
		UserText           string                    `json:"user_text"`
		Bond               []BondFixture             `json:"bond,omitempty"`
		Scenes             []SceneFixture            `json:"scenes,omitempty"`
		Workspaces         []TaskWorkspaceFixture    `json:"workspaces,omitempty"`
		Intents            []TaskIntentFixture       `json:"intents,omitempty"`
		Proposals          []ProposalFixture         `json:"proposals,omitempty"`
		Episodes           []MemoryFixture           `json:"episodes,omitempty"`
		ReadKeys           []string                  `json:"read_keys,omitempty"`
		UsedKeys           []string                  `json:"used_keys,omitempty"`
		DismissedKeys      []string                  `json:"dismissed_keys,omitempty"`
		Attention          map[string]attention.Tier `json:"attention,omitempty"`
		AttentionIdleTurns map[string]int            `json:"attention_idle_turns,omitempty"`
		Rules              []string                  `json:"semantic_rules"`
		ToolStarts         []string                  `json:"tool_starts,omitempty"`
		Answer             string                    `json:"answer"`
	}{
		CaseID: c.ID, Turn: turnIndex + 1, Description: c.Description, UserText: turn.UserText,
		Bond: c.Bond, Scenes: c.Scenes, Workspaces: c.Workspaces, Intents: c.Intents,
		Proposals: c.Proposals, Episodes: c.Episodes, ReadKeys: obs.ReadKeys,
		UsedKeys: obs.UsedKeys, DismissedKeys: obs.DismissedKeys, Attention: obs.Attention,
		AttentionIdleTurns: obs.AttentionIdleTurns, Rules: turn.Expect.SemanticRules,
		ToolStarts: obs.ToolStarts, Answer: obs.Answer,
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
		return SemanticResult{}, fmt.Errorf("decode attention semantic verdict: %w", err)
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
