package theater

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/workspace"
)

func appendProfessionalTools(out []tool.BaseTool, active ActorToolContext) ([]tool.BaseTool, error) {
	if active.Definition.Kind != RoleProfessional || active.Workspace == nil {
		return out, nil
	}
	allowed := map[string]bool{}
	for _, name := range active.Definition.ToolPolicy.Allowed {
		allowed[strings.TrimSpace(name)] = true
	}
	if allowed["list_workspace"] {
		item, err := utils.InferTool("list_workspace", "列出你被授权处理的 Workspace 文档候选；正文需要再读取。", func(context.Context, struct {
			Type   string `json:"type,omitempty"`
			Status string `json:"status,omitempty"`
			Limit  int    `json:"limit,omitempty"`
		}) (string, error) {
			docs, err := active.Workspace.List(workspace.ListFilter{Limit: 30})
			if err != nil {
				return "", err
			}
			cards := make([]workspace.Overview, 0, len(docs))
			for _, doc := range docs {
				cards = append(cards, workspace.ToOverview(doc))
			}
			raw, err := json.Marshal(map[string]any{"ok": true, "documents": cards})
			return string(raw), err
		})
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if allowed["read_workspace"] {
		item, err := utils.InferTool("read_workspace", "读取一个 Workspace 文档正文和修订历史。", func(_ context.Context, in struct {
			ID string `json:"id"`
		}) (string, error) {
			doc, err := active.Workspace.Get(strings.TrimSpace(in.ID))
			if err != nil {
				raw, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(raw), nil
			}
			raw, err := json.Marshal(map[string]any{"ok": true, "document": doc})
			return string(raw), err
		})
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if allowed["write_workspace"] {
		item, err := utils.InferTool("write_workspace", "创建或版本化更新 Workspace 文档；不能物理删除原稿。", func(_ context.Context, in struct {
			ID           string `json:"id,omitempty"`
			Type         string `json:"type,omitempty"`
			Status       string `json:"status,omitempty"`
			Title        string `json:"title,omitempty"`
			Summary      string `json:"summary,omitempty"`
			Body         string `json:"body,omitempty"`
			RevisionNote string `json:"revision_note,omitempty"`
		}) (string, error) {
			actor := "role:" + active.Definition.ID
			if strings.TrimSpace(in.ID) == "" {
				doc, err := active.Workspace.Create(workspace.Write{
					Type: workspace.NormalizeType(in.Type), Status: workspace.NormalizeStatus(in.Status),
					Title: strings.TrimSpace(in.Title), Summary: strings.TrimSpace(in.Summary),
					Body: strings.TrimSpace(in.Body), Actor: actor,
					RevisionNote: nonempty(in.RevisionNote, "created by professional role"),
				})
				if err != nil {
					return "", err
				}
				raw, err := json.Marshal(map[string]any{"ok": true, "created": true, "document": doc})
				return string(raw), err
			}
			update := workspace.Update{
				Title: strings.TrimSpace(in.Title), Summary: strings.TrimSpace(in.Summary),
				Body: strings.TrimSpace(in.Body), Actor: actor,
				RevisionNote: nonempty(in.RevisionNote, "updated by professional role"),
			}
			if strings.TrimSpace(in.Status) != "" {
				update.Status = workspace.NormalizeStatus(in.Status)
			}
			doc, err := active.Workspace.Update(strings.TrimSpace(in.ID), update)
			if err != nil {
				return "", err
			}
			raw, err := json.Marshal(map[string]any{"ok": true, "updated": true, "document": doc})
			return string(raw), err
		})
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func toolPolicyContains(policy RoleToolPolicy, name string) bool {
	for _, item := range policy.Allowed {
		if strings.TrimSpace(item) == name {
			return true
		}
	}
	return false
}
