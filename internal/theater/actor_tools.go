package theater

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/workspace"
)

type ActorToolContext struct {
	Scope      identity.TenantScope
	Definition RoleDefinition
	Instance   RoleInstance
	Session    RoleSession
	Worldlines []RoleWorldline
	Episodes   *memory.EpisodeStore
	Corpus     *CorpusStore
	Workspace  *workspace.Store
	Graph      RoleEpisodeGraph
}

type RoleEpisodeGraph interface {
	UpsertRoleEpisodePointer(ctx context.Context, scope identity.TenantScope, ep graph.RoleEpisodePointer) error
}

func ActorTools(active ActorToolContext) ([]tool.BaseTool, error) {
	if active.Episodes == nil {
		return nil, fmt.Errorf("actor episodes required")
	}
	allowedWorlds := map[string]bool{}
	maskedMemories := map[string]bool{}
	for _, world := range active.Worldlines {
		allowedWorlds[world.ID] = true
		for _, id := range world.MaskedMemoryIDs {
			maskedMemories[id] = true
		}
	}
	stateTool, err := utils.InferTool("get_role_state", "查看你当前能够感知的身份、场景、时间边界和世界线状态。", func(context.Context, struct{}) (string, error) {
		out, err := json.Marshal(map[string]any{
			"ok": true, "role": active.Definition.DisplayName, "knowledge_cutoff": active.Definition.KnowledgeCutoff,
			"scene": active.Instance.Scene, "state": active.Instance.State, "relationship": active.Instance.Relationship,
			"worldline_id": active.Session.WorldlineID,
		})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	searchTool, err := utils.InferTool("search_role_memories", "搜索你自己在当前人生中的经历候选；结果不含正文，依赖前必须读取。", func(ctx context.Context, in struct {
		Query string `json:"query"`
		Limit int    `json:"limit,omitempty"`
	}) (string, error) {
		limit := in.Limit
		if limit <= 0 || limit > 12 {
			limit = 8
		}
		var merged []memory.Episode
		seen := map[string]bool{}
		for _, world := range active.Worldlines {
			items, searchErr := active.Episodes.Search(ctx, active.Scope, memory.Query{
				Text: strings.TrimSpace(in.Query), Limit: limit, IncludeRole: true,
				RoleID: active.Definition.ID, RoleInstanceID: active.Instance.ID, WorldlineID: world.ID,
			})
			if searchErr != nil {
				return "", searchErr
			}
			for _, ep := range items {
				if maskedMemories[ep.ID] {
					continue
				}
				if !seen[ep.ID] {
					seen[ep.ID] = true
					merged = append(merged, ep)
				}
				if len(merged) >= limit {
					break
				}
			}
			if len(merged) >= limit {
				break
			}
		}
		cards := make([]map[string]any, 0, len(merged))
		for _, ep := range merged {
			cards = append(cards, map[string]any{
				"id": ep.ID, "kind": ep.Kind, "memory_class": ep.RoleMemoryClass,
				"summary": previewText(ep.Content, 120), "created_at": ep.CreatedAt,
			})
		}
		out, err := json.Marshal(map[string]any{"ok": true, "candidates": cards})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	readTool, err := utils.InferTool("read_role_memory", "按候选 id 读取你自己当前人生中的一条经历。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		ep, readErr := active.Episodes.Get(ctx, strings.TrimSpace(in.ID))
		if readErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "memory not found"})
			return string(out), nil
		}
		if ep.RoleID != active.Definition.ID || ep.RoleInstanceID != active.Instance.ID || !allowedWorlds[ep.WorldlineID] || maskedMemories[ep.ID] {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": "memory outside current life"})
			return string(out), nil
		}
		out, err := json.Marshal(map[string]any{"ok": true, "memory": ep})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	writeTool, err := utils.InferTool("write_role_memory", "仅在当前对话形成值得以后延续的经历或工作反馈时，写入你自己的角色记忆。不能创建 canonical 史实。", func(ctx context.Context, in struct {
		Content string `json:"content"`
		Why     string `json:"why,omitempty"`
		Class   string `json:"memory_class,omitempty"`
	}) (string, error) {
		content := strings.TrimSpace(in.Content)
		if content == "" {
			return "{\"ok\":false,\"error\":\"content required\"}", nil
		}
		class := MemoryClass(strings.TrimSpace(in.Class))
		mode := memory.ExperienceSimulatedRoleplay
		if active.Definition.Kind == RoleProfessional {
			class = MemoryOperational
			mode = memory.ExperienceDelegatedRole
		} else if class != MemorySimulated {
			class = MemorySimulated
		}
		ep, writeErr := active.Episodes.WriteEpisode(ctx, active.Scope, memory.EpisodeWrite{
			Kind: memory.EpisodeEvent, ExperienceMode: mode, Content: content, Why: strings.TrimSpace(in.Why),
			PersonIDs: []string{active.Scope.PersonID()}, SessionID: active.Session.ID,
			RoleID: active.Definition.ID, RoleInstanceID: active.Instance.ID, RoleSessionID: active.Session.ID,
			WorldlineID: active.Session.WorldlineID, RoleMemoryClass: string(class),
		})
		if writeErr != nil {
			return "", writeErr
		}
		if active.Graph != nil {
			_ = active.Graph.UpsertRoleEpisodePointer(ctx, active.Scope, graph.RoleEpisodePointer{
				ID: ep.ID, RoleID: ep.RoleID, RoleInstanceID: ep.RoleInstanceID, RoleSessionID: ep.RoleSessionID,
				WorldlineID: ep.WorldlineID, Kind: string(ep.Kind), MemoryClass: ep.RoleMemoryClass,
				Summary:   graph.SummaryFromContent(ep.Content, 160),
				CreatedAt: ep.CreatedAt, Status: string(ep.Status),
			})
		}
		out, err := json.Marshal(map[string]any{"ok": true, "id": ep.ID, "memory_class": class})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	base := []tool.BaseTool{stateTool, searchTool, readTool, writeTool}
	if active.Corpus != nil {
		searchBookTool, bookErr := utils.InferTool("search_book_passages", "在被授权给当前角色的一手资料和书籍中搜索原文候选；结果不含完整正文，引用或依赖前必须读取。", func(ctx context.Context, in struct {
			Query string `json:"query"`
			Limit int    `json:"limit,omitempty"`
		}) (string, error) {
			cards, err := active.Corpus.Search(ctx, active.Definition.CorpusRoleID, strings.TrimSpace(in.Query), SourceActor, in.Limit)
			if err != nil {
				return "", err
			}
			out, err := json.Marshal(map[string]any{"ok": true, "candidates": cards})
			return string(out), err
		})
		if bookErr != nil {
			return nil, bookErr
		}
		readBookTool, bookErr := utils.InferTool("read_book_passage", "按候选 id 读取当前角色被授权的一段原书正文，并保留书名、章节和页码。", func(ctx context.Context, in struct {
			ID string `json:"id"`
		}) (string, error) {
			chunk, err := active.Corpus.ReadChunk(ctx, strings.TrimSpace(in.ID))
			if err != nil || chunk.CorpusRoleID != active.Definition.CorpusRoleID || chunk.Audience != SourceActor {
				return `{"ok":false,"error":"passage outside current role corpus"}`, nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "passage": chunk})
			return string(out), err
		})
		if bookErr != nil {
			return nil, bookErr
		}
		base = append(base, searchBookTool, readBookTool)
	}
	return appendProfessionalTools(base, active)
}
