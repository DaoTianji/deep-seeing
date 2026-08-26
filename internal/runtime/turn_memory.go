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

const promptAgentRecallGuidance = `过去的具体事实、约定或经历会实质影响回答时，可以主动使用 search_episodes；需要核对某条原始经历时使用 read_episode。
润色、创作和一般知识任务通常不需要搜索。空结果是正常结果；同一轮最多换词重试一次，不要为了使用工具而搜索。
记忆是过去的证据，不是对用户的永久定义；用户当前的明确表达优先于旧记忆。`
