package runtime

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type turnMemoryContext struct {
	bondNorm        string
	recallGuidance  string
	recallLines     []string
	recallIDs       []string
	bondSlots       []string
	bondItemIDs     []string
	sceneIDs        []string
	bondPlaceholder bool
	normVersion     int64
}

func (s *Service) prepareTurnMemory(ctx context.Context, query string) turnMemoryContext {
	if s.RecallMode == RecallModeAgent {
		out := turnMemoryContext{recallGuidance: promptAgentContextSourceGuidance + "\n" + promptAgentAttentionGuidance + "\n" + promptAgentRecallGuidance}
		if s.Norms == nil {
			out.bondPlaceholder = true
			return out
		}
		snapshot, err := s.Norms.Snapshot(ctx)
		if err != nil {
			log.Printf("norm snapshot fallback: %v", err)
		}
		out.bondNorm = snapshot.Text
		out.bondPlaceholder = snapshot.Placeholder
		out.normVersion = snapshot.BondVersion
		return out
	}

	out := turnMemoryContext{}
	if s.SideQuery == nil {
		return out
	}
	recs, err := s.SideQuery.SelectForTurn(ctx, s.Scope, query, 5)
	if err != nil {
		log.Printf("side query skipped: %v", err)
		return out
	}
	for _, r := range recs {
		kind := r.Metadata["kind"]
		if kind == "bond" || kind == "scene_norm" {
			if out.bondNorm == "" {
				out.bondNorm = r.Content
			} else {
				out.bondNorm += "\n\n" + r.Content
			}
			if kind == "bond" {
				out.bondPlaceholder = r.Metadata["placeholder"] == "1"
				if slots := r.Metadata["bond_slots"]; slots != "" {
					out.bondSlots = strings.Split(slots, ",")
				}
				if ids := r.Metadata["bond_item_ids"]; ids != "" {
					out.bondItemIDs = strings.Split(ids, ",")
				}
			}
			if ids := r.Metadata["scene_ids"]; ids != "" {
				out.sceneIDs = append(out.sceneIDs, strings.Split(ids, ",")...)
			}
			if r.ID != "" {
				out.recallIDs = append(out.recallIDs, r.ID)
			}
			continue
		}
		tag := r.ID
		if tag == "" {
			tag = r.Key
		}
		if kind != "" {
			tag = kind + "/" + tag
		}
		if about := r.Metadata["about"]; about != "" {
			tag += " about:" + about
		}
		out.recallLines = append(out.recallLines, fmt.Sprintf("[%s] %s", tag, r.Content))
		if r.ID != "" {
			out.recallIDs = append(out.recallIDs, r.ID)
		}
	}
	return out
}

const promptAgentContextSourceGuidance = `按需选择零个或多个上下文来源，语义角色固定：Bond=关系 baseline（已自动提供），SceneNorm=场景 guidance，Workspace=当前 task，Intent=未来 plan，open Proposal=未确认 hypothesis，Episode=过去 evidence。候选卡不是正文；回答依赖内容前先用对应 read 核对，不能凭标题、状态或摘要补全。
认识论边界：用户当前明确表达优先；Intent active 或无 wake/完成记录都不能证明已执行或未完成；Proposal 不是事实；SceneNorm 不是全局真理。已读内容实质影响回答时，回答前公开声明：Episode 用 report_recall_evidence；SceneNorm/Proposal 用 report_context_use；Workspace/Intent 用一次 report_context_focus（可同时填两个 ID）。引用 Proposal 内容或其“未确认”性质也算 used；dismissed 只表示回答完全不依赖。先合并完成必要声明，再给面向用户的回答；公开轨迹不是隐藏思维，也不能代替回答。`

const promptAgentAttentionGuidance = `注意工作区只保存预计后续回合仍重要的有限焦点，不是事实来源或长期记忆；单轮材料通常不保存。center=核心，support=已读辅助，periphery=未展开弱线索。新增 center/support 必须本轮 read，新增 periphery 必须本轮成为 candidate；槽满时用 replace_source/replace_id 显式替换。idle_turns 只是未触碰时间，read 或公开处理会归零，不自动降级。当前表达和来源角色始终优先于注意层级。`

const promptAgentRecallGuidance = `任务快照只给 active Workspace/Intent 薄卡与会话焦点，不代表相关性。确需延续时 read；快照中没有目标时再 list。多个候选都可能符合含混指代时不要逐条猜，直接询问。正文影响回答时用 report_context_focus；新焦点先 read。continue=延续，switch=不同 ID 替换旧焦点，check=核对，compare=比较，clarify=仍有歧义并询问，clear=清除；工具说明是最终参数契约。用户明确要求读取或核对时必须同轮实际执行，不能只承诺。
过去事实、约定、经历或项目既有决定会实质改变回答时，自主 search_episodes；询问“我们为什么这样设计”时先搜项目经历，并区分召回结论与当前重构。润色、创作、计算和一般知识通常不搜。搜索只给候选，依赖前 read；空结果正常，同轮最多换词一次，不回退最近 Episode。
旧记忆不能永久定义用户。回答过去特定事实不得超出已采用 Episode；不能用当前能力或常识补历史空白。指代不明且无证据时承认无法确定并沿用用户原词询问。最终回答依赖已读 Episode 时，回答前用 report_recall_evidence 标 used；完全不依赖的主动排除候选才标 dismissed，未处理候选无需声明。`
