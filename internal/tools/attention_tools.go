package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/observe"
)

func appendAttentionTool(toolsOut []tool.BaseTool, deps Deps) ([]tool.BaseTool, error) {
	if deps.Attention == nil || !strings.EqualFold(strings.TrimSpace(deps.RecallMode), "agent") {
		return toolsOut, nil
	}
	manage, err := utils.InferTool(
		"manage_attention",
		"管理仅在当前会话存在的注意工作区。只把预计后续回合仍会重要的已出现项目放入 center/support/periphery；单轮即结束的材料无需保存。center/support 新项目必须先读取，periphery 至少必须是本轮候选。槽位满时系统不会自动淘汰，必须用 replace_source/replace_id 明确选择被替换项目。它不证明事实、不代替 read/use/focus 声明，也不写长期记忆。",
		func(ctx context.Context, in manageAttentionInput) (string, error) {
			decisions := make([]attention.Decision, 0, len(in.Decisions))
			before := deps.Attention.Snapshot(deps.SessionID)
			beforeItems := attentionItemsByKey(before)
			for _, raw := range in.Decisions {
				decision := attention.Decision{
					Source: contextsource.Source(strings.ToLower(strings.TrimSpace(raw.Source))),
					ID:     strings.TrimSpace(raw.ID), Target: attention.Tier(strings.ToLower(strings.TrimSpace(raw.Target))),
					ReplaceSource: contextsource.Source(strings.ToLower(strings.TrimSpace(raw.ReplaceSource))),
					ReplaceID:     strings.TrimSpace(raw.ReplaceID),
				}
				key := attentionKey(decision.Source, decision.ID)
				existing, exists := beforeItems[key]
				switch decision.Target {
				case attention.Center, attention.Support:
					if (!exists || existing.Tier == attention.Periphery) && !observe.ContextReadKnown(ctx, decision.Source, decision.ID) {
						return attentionToolError("center/support requires a successful read in this turn unless the item is already center/support"), nil
					}
				case attention.Periphery:
					if !exists && !observe.ContextCandidateKnown(ctx, decision.Source, decision.ID) {
						return attentionToolError("new periphery item must be a candidate in this turn"), nil
					}
				case attention.Drop:
				default:
					return attentionToolError("target must be center, support, periphery, or drop"), nil
				}
				decisions = append(decisions, decision)
			}
			after, err := deps.Attention.Apply(deps.SessionID, decisions)
			if err != nil {
				return attentionToolError(err.Error()), nil
			}
			for _, decision := range decisions {
				from := attention.Tier("")
				if existing, ok := beforeItems[attentionKey(decision.Source, decision.ID)]; ok {
					from = existing.Tier
				}
				observe.RecordAttentionDecision(ctx, observe.AttentionDecisionTrace{
					Source: decision.Source, ID: decision.ID, From: from, To: decision.Target,
					ReplacedSource: decision.ReplaceSource, ReplacedID: decision.ReplaceID,
				})
			}
			out, err := json.Marshal(map[string]any{"ok": true, "attention": after})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	return append(toolsOut, manage), nil
}

type manageAttentionInput struct {
	Decisions []attentionDecisionInput `json:"decisions" jsonschema:"description=本次原子化注意层级调整；同一项目只能出现一次"`
}

type attentionDecisionInput struct {
	Source        string `json:"source" jsonschema:"description=scene_norm|workspace|intent|proposal|episode；Bond 是自动 baseline 不参与槽位竞争"`
	ID            string `json:"id"`
	Target        string `json:"target" jsonschema:"description=center|support|periphery|drop"`
	ReplaceSource string `json:"replace_source,omitempty" jsonschema:"description=目标层已满时，明确指定要被替换项目的来源"`
	ReplaceID     string `json:"replace_id,omitempty" jsonschema:"description=目标层已满时，明确指定要被替换项目的 ID"`
}

func attentionItemsByKey(snapshot attention.Snapshot) map[string]attention.Item {
	out := make(map[string]attention.Item, len(snapshot.Items))
	for _, item := range snapshot.Items {
		out[attentionKey(item.Source, item.ID)] = item
	}
	return out
}

func attentionKey(source contextsource.Source, id string) string {
	return string(source) + ":" + strings.TrimSpace(id)
}

func attentionToolError(message string) string {
	out, _ := json.Marshal(map[string]any{"ok": false, "error": message})
	return string(out)
}
