package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/workspace"
)

func appendWorkspaceTools(toolsOut []tool.BaseTool, deps Deps) ([]tool.BaseTool, error) {
	if deps.Workspace == nil {
		return toolsOut, nil
	}
	store := deps.Workspace

	listWS, err := utils.InferTool(
		"list_workspace",
		"列出 Workspace 未完成思考：question / writing / research / project。",
		func(ctx context.Context, in listWorkspaceInput) (string, error) {
			limit := in.Limit
			if limit <= 0 {
				limit = 30
			}
			filter := workspace.ListFilter{Limit: limit}
			if typ := strings.TrimSpace(in.Type); typ != "" {
				filter.Type = workspace.NormalizeType(typ)
			}
			if st := strings.TrimSpace(in.Status); st != "" {
				filter.Status = workspace.NormalizeStatus(st)
			}
			list, err := store.List(filter)
			if err != nil {
				return "", err
			}
			ovs := make([]workspace.Overview, 0, len(list))
			for _, d := range list {
				ovs = append(ovs, workspace.ToOverview(d))
			}
			out, err := json.Marshal(map[string]any{"ok": true, "items": ovs})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, listWS)

	readWS, err := utils.InferTool(
		"read_workspace",
		"读取一条 Workspace 文档全文、修订史与关联 Episode。",
		func(ctx context.Context, in readWorkspaceInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			if id == "" {
				return `{"ok":false,"error":"id 不能为空"}`, nil
			}
			d, err := store.Get(id)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "document": d})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, readWS)

	writeWS, err := utils.InferTool(
		"write_workspace",
		"创建或更新 Workspace 文档（未完成思考）。更新会追加 revision。不是 Self 结晶，也不是 Episode。",
		func(ctx context.Context, in writeWorkspaceInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			actor := "tool"
			if id == "" {
				body := strings.TrimSpace(in.Body)
				title := strings.TrimSpace(in.Title)
				if body == "" && title == "" {
					return `{"ok":false,"error":"title 或 body 不能为空"}`, nil
				}
				d, err := store.Create(workspace.Write{
					Type:   workspace.NormalizeType(in.Type),
					Status: workspace.NormalizeStatus(in.Status),
					Title:  title, Summary: strings.TrimSpace(in.Summary), Body: body,
					Actor: actor, RevisionNote: firstNonEmptyStr(strings.TrimSpace(in.RevisionNote), "created"),
				})
				if err != nil {
					out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
					return string(out), nil
				}
				out, err := json.Marshal(map[string]any{"ok": true, "created": true, "document": d})
				return string(out), err
			}
			u := workspace.Update{
				Title: strings.TrimSpace(in.Title), Summary: strings.TrimSpace(in.Summary),
				Body: strings.TrimSpace(in.Body), Actor: actor,
				RevisionNote: strings.TrimSpace(in.RevisionNote),
			}
			if st := strings.TrimSpace(in.Status); st != "" {
				u.Status = workspace.NormalizeStatus(st)
			}
			d, err := store.Update(id, u)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "updated": true, "document": d})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, writeWS)

	linkWS, err := utils.InferTool(
		"link_workspace_episode",
		"把一条 Episode 关联到 Workspace 文档（去重）。",
		func(ctx context.Context, in linkWorkspaceInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			eid := strings.TrimSpace(in.EpisodeID)
			if id == "" || eid == "" {
				return `{"ok":false,"error":"id 与 episode_id 不能为空"}`, nil
			}
			d, err := store.LinkEpisode(id, eid)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "document": d})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	return append(toolsOut, linkWS), nil
}

type listWorkspaceInput struct {
	Type   string `json:"type,omitempty" jsonschema:"description=question|writing|research|project；空则全部"`
	Status string `json:"status,omitempty" jsonschema:"description=open|in_progress|paused|done|archived"`
	Limit  int    `json:"limit,omitempty"`
}

type readWorkspaceInput struct {
	ID string `json:"id" jsonschema:"description=Workspace 文档 id，如 wq_… / ww_…"`
}

type writeWorkspaceInput struct {
	ID           string `json:"id,omitempty" jsonschema:"description=空则创建；有则更新"`
	Type         string `json:"type,omitempty" jsonschema:"description=创建时：question|writing|research|project"`
	Status       string `json:"status,omitempty" jsonschema:"description=open|in_progress|paused|done|archived"`
	Title        string `json:"title,omitempty"`
	Summary      string `json:"summary,omitempty"`
	Body         string `json:"body,omitempty"`
	RevisionNote string `json:"revision_note,omitempty" jsonschema:"description=更新时的修订说明"`
}

type linkWorkspaceInput struct {
	ID        string `json:"id" jsonschema:"description=Workspace 文档 id"`
	EpisodeID string `json:"episode_id" jsonschema:"description=Episode id"`
}
