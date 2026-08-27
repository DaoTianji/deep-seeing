package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
)

func appendMultiSourceContextTools(toolsOut []tool.BaseTool, deps Deps, scope identity.TenantScope, agentMode bool) ([]tool.BaseTool, error) {
	if !agentMode {
		return toolsOut, nil
	}

	if deps.Proposals != nil {
		listProposals, err := utils.InferTool(
			"list_proposals",
			"列出当前对话者尚未裁决的 Bond Proposal 假设候选卡。Proposal 不是事实；已采纳内容已进入 Bond，已拒绝内容不会返回。",
			func(ctx context.Context, in listProposalsInput) (string, error) {
				limit := in.Limit
				if limit <= 0 || limit > 50 {
					limit = 12
				}
				items, err := deps.Proposals.ListOpen(ctx, scope, scope.PersonID(), 1000)
				if err != nil {
					observe.RecordContextCandidate(ctx, observe.ContextCandidateTrace{
						Source: contextsource.Proposal, Operation: "list", Error: err.Error(),
					})
					return "", err
				}
				cards := make([]contextsource.Candidate, 0, len(items))
				ids := make([]string, 0, len(items))
				for _, item := range items {
					if memory.NormalizeProposalKind(string(item.Kind)) != memory.ProposalKindBond || item.Status != memory.ProposalOpen {
						continue
					}
					cards = append(cards, contextsource.Candidate{
						Source: contextsource.Proposal, ID: item.ID, Kind: string(item.Kind),
						Title: item.Field, Preview: candidatePreview(item.SuggestedText, 120),
						Role: contextsource.Hypothesis, Status: string(item.Status),
						UpdatedAt: item.UpdatedAt, ReadRequired: true,
						Metadata: map[string]any{
							"hypothesis": item.Hypothesis,
							"mode":       item.Mode,
						},
					})
					ids = append(ids, item.ID)
					if len(cards) >= limit {
						break
					}
				}
				observe.RecordContextCandidate(ctx, observe.ContextCandidateTrace{
					Source: contextsource.Proposal, Operation: "list", ResultIDs: ids,
				})
				out, err := json.Marshal(map[string]any{"ok": true, "candidates": cards})
				return string(out), err
			},
		)
		if err != nil {
			return nil, err
		}
		toolsOut = append(toolsOut, listProposals)

		readProposal, err := utils.InferTool(
			"read_proposal",
			"读取一条未裁决的 Bond Proposal。它只能作为待验证假设，不能当作用户事实或 Episode 证据。",
			func(ctx context.Context, in readProposalInput) (string, error) {
				id := strings.TrimSpace(in.ID)
				if id == "" {
					observe.RecordContextRead(ctx, observe.ContextReadTrace{Source: contextsource.Proposal, Error: "id 不能为空"})
					return `{"ok":false,"error":"id 不能为空"}`, nil
				}
				item, err := deps.Proposals.Get(ctx, id)
				if err != nil || item.PersonID != scope.PersonID() ||
					memory.NormalizeProposalKind(string(item.Kind)) != memory.ProposalKindBond ||
					item.Status != memory.ProposalOpen {
					reason := "proposal not found"
					observe.RecordContextRead(ctx, observe.ContextReadTrace{Source: contextsource.Proposal, ID: id, Error: reason})
					out, _ := json.Marshal(map[string]any{"ok": false, "error": reason})
					return string(out), nil
				}
				observe.RecordContextRead(ctx, observe.ContextReadTrace{Source: contextsource.Proposal, ID: id})
				out, err := json.Marshal(map[string]any{"ok": true, "proposal": item, "epistemic_role": contextsource.Hypothesis})
				return string(out), err
			},
		)
		if err != nil {
			return nil, err
		}
		toolsOut = append(toolsOut, readProposal)
	}

	if deps.Scenes == nil && deps.Proposals == nil {
		return toolsOut, nil
	}
	reportUse, err := utils.InferTool(
		"report_context_use",
		"公开声明本轮如何处理已出现的 SceneNorm 指导或 Proposal 假设。used 必须先读取；dismissed 必须给结构化原因。不会写长期记忆。",
		func(ctx context.Context, in reportContextUseInput) (string, error) {
			source := contextsource.Source(strings.ToLower(strings.TrimSpace(in.Source)))
			if source != contextsource.SceneNorm && source != contextsource.Proposal {
				return contextUseError(fmt.Errorf("source must be scene_norm or proposal"))
			}
			disposition := strings.ToLower(strings.TrimSpace(in.Disposition))
			if disposition != "used" && disposition != "dismissed" {
				return contextUseError(fmt.Errorf("disposition must be used or dismissed"))
			}
			event := observe.ContextUseTrace{
				Source: source, ID: in.ID, Role: contextsource.RoleFor(source),
				Disposition: disposition, ReasonCode: in.ReasonCode,
			}
			if err := observe.RecordContextUse(ctx, event); err != nil {
				return contextUseError(err)
			}
			out, err := json.Marshal(map[string]any{"ok": true, "context_use": event})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	return append(toolsOut, reportUse), nil
}

func contextUseError(err error) (string, error) {
	out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	return string(out), nil
}

type listProposalsInput struct {
	Limit int `json:"limit,omitempty"`
}

type readProposalInput struct {
	ID string `json:"id"`
}

type reportContextUseInput struct {
	Source      string `json:"source" jsonschema:"description=scene_norm|proposal"`
	ID          string `json:"id"`
	Disposition string `json:"disposition" jsonschema:"description=used|dismissed"`
	ReasonCode  string `json:"reason_code,omitempty" jsonschema:"description=dismissed 必填：irrelevant|not_applicable|conflicting|stale|insufficient|superseded|duplicate|unconfirmed"`
}
