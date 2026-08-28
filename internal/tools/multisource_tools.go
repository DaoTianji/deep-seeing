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
			"列出未裁决 Bond Proposal 候选；它们是 hypothesis，不是事实。",
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
			"读取未裁决 Proposal；只能作为 hypothesis，不能当作事实或 Episode 证据。",
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
		"声明 SceneNorm/Proposal 的 used 或 dismissed；used 须 read，dismissed 须给 reason；不写 LTM。",
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
