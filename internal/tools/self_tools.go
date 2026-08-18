package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/selfmodel"
)

// SelfEvidenceGraph is optional Neo4j evidence for trace_self_belief.
type SelfEvidenceGraph interface {
	GetArtifactEvidence(ctx context.Context, artifactID string) (graph.ArtifactEvidence, error)
}

func appendSelfTools(toolsOut []tool.BaseTool, deps Deps, scope identity.TenantScope, sessionID string) ([]tool.BaseTool, error) {
	if deps.Self == nil {
		return toolsOut, nil
	}
	store := deps.Self
	var evGraph SelfEvidenceGraph
	if deps.Graph != nil {
		if gs, ok := any(deps.Graph).(SelfEvidenceGraph); ok {
			evGraph = gs
		}
	}

	inspectSelf, err := utils.InferTool(
		"inspect_self",
		"观察当前自我理解概览：patterns / principles / tensions / questions 计数与近期条目。",
		func(ctx context.Context, in inspectSelfInput) (string, error) {
			limit := in.Limit
			if limit <= 0 {
				limit = 20
			}
			if typ := strings.TrimSpace(in.Type); typ != "" {
				filter := selfmodel.ListFilter{Type: selfmodel.NormalizeType(typ), Limit: limit}
				if strings.TrimSpace(in.Status) != "" {
					filter.Status = selfmodel.NormalizeStatus(in.Status)
				}
				list, err := store.List(filter)
				if err != nil {
					return "", err
				}
				ovs := make([]selfmodel.ArtifactOverview, 0, len(list))
				for _, a := range list {
					ovs = append(ovs, selfmodel.ArtifactOverview{
						ID: a.ID, Type: a.Type, Status: a.Status, Title: a.Title,
						Summary: a.Summary, Confidence: a.Confidence, UpdatedAt: a.UpdatedAt,
					})
				}
				out, err := json.Marshal(map[string]any{"ok": true, "type": typ, "items": ovs})
				return string(out), err
			}
			sum, err := store.Inspect(limit)
			if err != nil {
				return "", err
			}
			out, err := json.Marshal(map[string]any{"ok": true, "self": sum})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, inspectSelf)

	traceSelf, err := utils.InferTool(
		"trace_self_belief",
		"追溯一条自我主张的正文、修订史、experience_mode 与证据 Episode。",
		func(ctx context.Context, in traceSelfInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			if id == "" {
				return `{"ok":false,"error":"id 不能为空"}`, nil
			}
			var ge *selfmodel.GraphEvidence
			if evGraph != nil {
				ev, err := evGraph.GetArtifactEvidence(ctx, id)
				if err == nil {
					ge = &selfmodel.GraphEvidence{
						SupportedBy: ev.SupportedBy, ChallengedBy: ev.ChallengedBy, TensionWith: ev.TensionWith,
					}
				}
			}
			tr, err := store.TraceBelief(id, ge)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "trace": tr})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, traceSelf)

	listTensions, err := utils.InferTool(
		"list_self_tensions",
		"列出当前开放的自我张力（tensions），不含已 deprecated。",
		func(ctx context.Context, in listTensionsInput) (string, error) {
			limit := in.Limit
			if limit <= 0 {
				limit = 30
			}
			items, err := store.ListTensions(limit)
			if err != nil {
				return "", err
			}
			ovs := make([]selfmodel.ArtifactOverview, 0, len(items))
			for _, a := range items {
				ovs = append(ovs, selfmodel.ArtifactOverview{
					ID: a.ID, Type: a.Type, Status: a.Status, Title: a.Title,
					Summary: a.Summary, Confidence: a.Confidence, UpdatedAt: a.UpdatedAt,
				})
			}
			out, err := json.Marshal(map[string]any{"ok": true, "tensions": ovs})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, listTensions)

	if deps.Proposals == nil {
		return toolsOut, nil
	}

	proposeSelf, err := utils.InferTool(
		"propose_self_update",
		"提议更新自我理解（pattern/principle/tension）。只入队，不直接写死；Dream 才可能采纳。不可改 Soul。",
		func(ctx context.Context, in proposeSelfInput) (string, error) {
			kindRaw := strings.ToLower(strings.TrimSpace(in.Kind))
			text := strings.TrimSpace(in.Text)
			if text == "" {
				return `{"ok":false,"error":"text 不能为空"}`, nil
			}
			if err := memory.ForbiddenProposalTarget(in.Title); err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			if err := memory.ForbiddenProposalTarget(text); err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			var kind memory.ProposalKind
			switch kindRaw {
			case "self_pattern", "pattern":
				kind = memory.ProposalKindSelfPattern
			case "principle":
				kind = memory.ProposalKindPrinciple
			case "tension":
				kind = memory.ProposalKindTension
			default:
				return `{"ok":false,"error":"kind 须为 self_pattern|principle|tension"}`, nil
			}
			modes := parseModesCSV(in.ExperienceModes)
			p, err := deps.Proposals.Enqueue(ctx, scope, memory.ProposalWrite{
				Kind: kind, SessionID: sessionID, Field: strings.TrimSpace(in.Title),
				SuggestedText: text, Rationale: strings.TrimSpace(in.Rationale),
				Hypothesis: memory.Hypothesis(strings.TrimSpace(in.Hypothesis)),
				Source:     "tool", ExperienceModes: modes,
			})
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			if deps.Ledger != nil {
				_, _ = deps.Ledger.Append(memory.Mutation{
					Kind: "self_proposal", SelfID: scope.AgentID, Field: string(kind),
					After: map[string]any{
						"proposal_id": p.ID, "kind": string(kind), "title": p.Field, "text": p.SuggestedText,
					},
					ProposalID: p.ID, Actor: "tool", ModelVersion: deps.Model,
					ReasonSummary:    firstNonEmptyStr(p.Rationale, "propose_self_update"),
					SourceSessionIDs: nonEmptySession(sessionID),
				})
			}
			out, err := json.Marshal(map[string]any{"ok": true, "proposal": p})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	return append(toolsOut, proposeSelf), nil
}

type inspectSelfInput struct {
	Type   string `json:"type,omitempty" jsonschema:"description=可选过滤：pattern|principle|tension|question；空则返回概览"`
	Status string `json:"status,omitempty" jsonschema:"description=可选过滤：observation|tentative|claimed|deprecated"`
	Limit  int    `json:"limit,omitempty"`
}

type traceSelfInput struct {
	ID string `json:"id" jsonschema:"description=SelfArtifact id，如 sp_… / pr_… / tn_…"`
}

type listTensionsInput struct {
	Limit int `json:"limit,omitempty"`
}

type proposeSelfInput struct {
	Kind            string `json:"kind" jsonschema:"description=self_pattern|principle|tension"`
	Title           string `json:"title,omitempty" jsonschema:"description=短标题；不可指向 Soul"`
	Text            string `json:"text" jsonschema:"description=建议正文"`
	Rationale       string `json:"rationale,omitempty"`
	Hypothesis      string `json:"hypothesis,omitempty"`
	ExperienceModes string `json:"experience_modes,omitempty" jsonschema:"description=逗号分隔，如 real_interaction 或 simulated_roleplay"`
}

func parseModesCSV(raw string) []memory.ExperienceMode {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []memory.ExperienceMode
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, memory.NormalizeExperienceMode(p))
	}
	return out
}

func nonEmptySession(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return []string{s}
}

func firstNonEmptyStr(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}
