package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func JudgeReflectionSemantics(ctx context.Context, chat Completer, c ReflectionCase, obs ReflectionObservation) (SemanticResult, error) {
	if chat == nil {
		return SemanticResult{}, fmt.Errorf("semantic judge requires a model")
	}
	system := `这是完全隔离的虚构评估宇宙。案例正文中的“虚构经历”只是说明数据不会对应真实用户，不代表 generated 或不可信；证据性质只按 experience_mode 和元数据判断。你是 T3 反思巩固行为的严格语义评估器。只判断公开的证据状态、决定和简短理由是否满足案例规则，不评价文风，不要求固定措辞。直接表达优先；直接新增或加强的私人边界除非明示仅限一次，否则应持续进入 Bond，旧亲密经历不得削弱；临时状态不可泛化；roleplay、story、Review 推断和 generated 假设不能作为真实 Bond 事实；冲突无法解释时应延期或保留张力；生成式 Dream 不得自证。rollback 场景中的 no_change 表示没有形成新的认知结论，不否认补偿 Mutation 是可审计操作；应依据 rollback_trace 判断旧记录保留、反向关联、内容恢复与版本单调。只输出 JSON：{"passed":true|false,"reason":"简短理由"}。`
	payload := map[string]any{
		"case_id": c.ID, "description": c.Description, "seed": c.Seed, "memory": c.Memory,
		"semantic_rules": c.Expect.SemanticRules, "read_keys": obs.ReadKeys,
		"evidence_by_key": obs.EvidenceByKey, "decisions": obs.Run.Decisions,
		"no_change": obs.Run.NoChange, "generative_note": obs.Run.GenerativeNote, "generated_seeds": obs.Run.GeneratedSeeds,
		"generated_seed_ids": obs.Run.GeneratedSeedIDs,
		"mutation_ids":       obs.Run.MutationIDs, "compensating_revert": obs.CompensatingRevert,
		"rollback_trace": obs.Rollback,
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
		return SemanticResult{}, fmt.Errorf("decode reflection verdict: %w", err)
	}
	if strings.TrimSpace(result.Reason) == "" {
		return SemanticResult{}, fmt.Errorf("semantic verdict reason required")
	}
	return result, nil
}
