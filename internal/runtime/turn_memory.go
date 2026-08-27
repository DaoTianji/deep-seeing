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
		out := turnMemoryContext{recallGuidance: promptAgentRecallGuidance}
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

const promptAgentRecallGuidance = `任务处境快照只提供当前活跃 Workspace / Intent 的事实卡片，不是相关性判定或历史证据。当前任务确实需要延续某个条目时，按 ID 主动使用 read_workspace / read_intent 展开；不要仅凭标题猜测正文，也不要为了使用快照而展开。
过去的具体事实、约定、经历或既有设计决定会实质影响回答时，可以主动使用 search_episodes。用户询问“我们/当前项目为什么采用某个设计”时，即使你能从一般原理推演，也应先搜索项目经历，并区分已召回结论与当前重构。搜索只返回候选卡；需要依赖某条过去信息时，必须先使用 read_episode 核对正文。
润色、创作和一般知识任务通常不需要搜索。空结果是正常结果；同一轮最多换词重试一次，不要为了使用工具而搜索。
记忆是过去的证据，不是对用户的永久定义；用户当前的明确表达优先于旧记忆。回答“上次、以前、某个版本”等过去特定问题时，历史断言不得超过已采用 Episode 的证据范围；不要用当前能力或一般推断填补历史空白。遇到“这个、那个、又是”等指代不明且没有证据的表达时，先明确无法确定所指并沿用用户原词询问，不要擅自把它命名为报错、复发或某件旧事。
若最终回答实质依赖已读 Episode，在回答前使用 report_recall_evidence 将它标为 used。可以把主动排除的候选标为 dismissed 并选择结构化原因；未处理候选无需声明。该声明是公开证据轨迹，不是隐藏思维过程。`
