package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/body"
	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/selfmodel"
	"deep-seeing/internal/workspace"
	"deep-seeing/internal/world"
)

// GraphStore is the optional L2 surface for agent tools.
// Note: full PatchBond is NOT exposed to the main agent — only explicit facts + propose.
type GraphStore interface {
	GetBond(ctx context.Context, scope identity.TenantScope, personID string) (graph.Bond, error)
	PatchBond(ctx context.Context, scope identity.TenantScope, personID string, patch graph.BondPatch) (graph.Bond, error)
	UpsertEpisodePointer(ctx context.Context, scope identity.TenantScope, ep graph.EpisodePointer) error
	UpsertCalls(ctx context.Context, scope identity.TenantScope, personID, name, sourceEpisodeID string) error
	MarkEpisodeStatus(ctx context.Context, episodeID, status, reason string) error
}

// Deps wires tool backends.
type Deps struct {
	Scope     identity.TenantScope
	Episodes  *memory.EpisodeStore
	Graph     GraphStore            // optional
	Proposals *memory.ProposalStore // optional
	Self      *selfmodel.Store      // optional — SelfArtifact file store
	Workspace *workspace.Store      // optional — unfinished thinking
	Intents   *intent.Store         // optional — agency intents
	World     *world.Gateway        // optional — web gateway
	Ledger    *memory.MutationLedger
	SessionID string
	Model     string
	Stores    map[string]string // stm/episode/graph availability
	FirstBoot bool
}

// All returns agent tools: body/capabilities + memory (+ restricted bond).
func All(deps Deps) ([]tool.BaseTool, error) {
	if deps.Episodes == nil {
		return nil, fmt.Errorf("episodes required")
	}
	scope := deps.Scope
	if err := scope.Validate(); err != nil {
		scope = identity.LocalCLI()
	}
	sessionID := deps.SessionID
	store := deps.Episodes
	g := deps.Graph
	hasGraph := g != nil
	hasProps := deps.Proposals != nil
	hasSelf := deps.Self != nil
	hasWorkspace := deps.Workspace != nil
	hasIntents := deps.Intents != nil
	hasWorld := deps.World != nil

	toolsOut := []tool.BaseTool{}

	inspect, err := utils.InferTool(
		"inspect_runtime",
		"查看当前 Runtime：时间、对话者、模型版本、存储可用性与持久性约定。",
		func(ctx context.Context, _ struct{}) (string, error) {
			snap := body.BuildSnapshot(scope, sessionID, deps.Model, deps.Stores, deps.FirstBoot)
			out, err := json.Marshal(map[string]any{"ok": true, "runtime": snap})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, inspect)

	listCap, err := utils.InferTool(
		"list_capabilities",
		"列出可用能力摘要（不要依赖 System Prompt 里的完整工具堆）。",
		func(ctx context.Context, _ struct{}) (string, error) {
			out, err := json.Marshal(map[string]any{"ok": true, "capabilities": body.Catalog(hasGraph, hasProps, hasSelf, hasWorkspace, hasIntents, hasWorld)})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, listCap)

	toolHelp, err := utils.InferTool(
		"tool_help",
		"查询单个工具的用途、持久性与副作用。",
		func(ctx context.Context, in toolHelpInput) (string, error) {
			c, ok := body.FindCapability(in.Name, hasGraph, hasProps, hasSelf, hasWorkspace, hasIntents, hasWorld)
			if !ok {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": "unknown tool"})
				return string(out), nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "capability": c})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, toolHelp)

	getTime, err := utils.InferTool(
		"get_time",
		"查看当前时间与时区。",
		func(ctx context.Context, _ struct{}) (string, error) {
			tz := "Asia/Shanghai"
			if v := strings.TrimSpace(os.Getenv("TZ")); v != "" {
				tz = v
			}
			loc, err := time.LoadLocation(tz)
			now := time.Now()
			if err == nil {
				now = now.In(loc)
			}
			out, err := json.Marshal(map[string]any{"ok": true, "now": now.Format(time.RFC3339), "timezone": tz})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, getTime)

	writeEp, err := utils.InferTool(
		"write_episode",
		"仅在你认为这件事对自己有长期价值时调用：留下经历/约定/边界/观察，或关于自己的理解（kind=self_note / about=self）。故事或角色代入必须设 experience_mode=simulated_roleplay|story_reading。不必每轮都写。",
		func(ctx context.Context, in writeEpisodeInput) (string, error) {
			content := strings.TrimSpace(in.Content)
			if content == "" {
				return `{"ok":false,"error":"content 不能为空"}`, nil
			}
			kind := memory.NormalizeEpisodeKind(in.Kind)
			mode := memory.NormalizeExperienceMode(in.ExperienceMode)
			aboutSelf, persons := memory.ResolveEpisodeSubjects(scope, kind, in.About)
			ep, err := store.WriteEpisode(ctx, scope, memory.EpisodeWrite{
				Kind: kind, ExperienceMode: mode, Content: content,
				Why: strings.TrimSpace(in.Why), PersonIDs: persons, SessionID: sessionID,
			})
			if err != nil {
				return "", err
			}
			if g != nil {
				docURI := filepath.ToSlash(filepath.Join("by_id", ep.ID+".md"))
				if err := g.UpsertEpisodePointer(ctx, scope, graph.EpisodePointer{
					ID: ep.ID, Kind: string(ep.Kind), Summary: graph.SummaryFromContent(ep.Content, 160),
					DocURI: docURI, SessionID: ep.SessionID, CreatedAt: ep.CreatedAt,
					PersonIDs: ep.PersonIDs, AboutSelf: aboutSelf,
					ExperienceMode: string(ep.ExperienceMode), Status: string(ep.Status),
				}); err != nil {
					log.Printf("graph upsert episode: %v", err)
				}
			}
			out, err := json.Marshal(map[string]any{
				"ok": true, "id": ep.ID, "kind": ep.Kind, "experience_mode": ep.ExperienceMode,
				"about_self": aboutSelf, "person_ids": ep.PersonIDs,
			})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, writeEp)

	readEp, err := utils.InferTool(
		"read_episode",
		"按 episode id 读取经历正文（含归档/失效条目）。",
		func(ctx context.Context, in readEpisodeInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			if id == "" {
				return `{"ok":false,"error":"id 不能为空"}`, nil
			}
			ep, err := store.Get(ctx, id)
			if err != nil {
				return "", err
			}
			out, err := json.Marshal(map[string]any{"ok": true, "episode": ep})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, readEp)

	searchEp, err := utils.InferTool(
		"search_episodes",
		"按关键词检索经历（默认不含 archived/invalid）。",
		func(ctx context.Context, in searchEpisodeInput) (string, error) {
			limit := in.Limit
			if limit <= 0 {
				limit = 8
			}
			eps, err := store.Search(ctx, scope, memory.Query{Text: strings.TrimSpace(in.Query), Limit: limit})
			if err != nil {
				return "", err
			}
			out, err := json.Marshal(map[string]any{"ok": true, "episodes": eps})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, searchEp)

	archiveEp, err := utils.InferTool(
		"archive_episode",
		"软忘记：将经历标为 archived（不物理删除；默认召回跳过）。",
		func(ctx context.Context, in statusEpisodeInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			if id == "" {
				return `{"ok":false,"error":"id 不能为空"}`, nil
			}
			ep, err := store.SetStatus(ctx, id, memory.EpisodeArchived, "")
			if err != nil {
				return "", err
			}
			if g != nil {
				_ = g.MarkEpisodeStatus(ctx, id, "archived", "")
			}
			out, err := json.Marshal(map[string]any{"ok": true, "episode": ep})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, archiveEp)

	invalidateEp, err := utils.InferTool(
		"invalidate_episode",
		"判定经历无效（保留证据与原因；默认召回跳过；图侧降权）。",
		func(ctx context.Context, in statusEpisodeInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			if id == "" {
				return `{"ok":false,"error":"id 不能为空"}`, nil
			}
			reason := strings.TrimSpace(in.Reason)
			if reason == "" {
				return `{"ok":false,"error":"reason 不能为空"}`, nil
			}
			ep, err := store.SetStatus(ctx, id, memory.EpisodeInvalid, reason)
			if err != nil {
				return "", err
			}
			if g != nil {
				_ = g.MarkEpisodeStatus(ctx, id, "invalid", reason)
			}
			out, err := json.Marshal(map[string]any{"ok": true, "episode": ep})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, invalidateEp)

	toolsOut, err = appendSelfTools(toolsOut, deps, scope, sessionID)
	if err != nil {
		return nil, err
	}

	toolsOut, err = appendWorkspaceTools(toolsOut, deps)
	if err != nil {
		return nil, err
	}

	toolsOut, err = appendIntentTools(toolsOut, deps, scope.AgentID)
	if err != nil {
		return nil, err
	}

	toolsOut, err = appendWorldTools(toolsOut, deps)
	if err != nil {
		return nil, err
	}

	if hasProps {
		proposeBond, err := utils.InferTool(
			"propose_bond_update",
			"提议改变长期理解（性格/边界/策略等）。只入队，不直接写死；Dream 才可能采纳。",
			func(ctx context.Context, in proposeBondInput) (string, error) {
				field := strings.TrimSpace(in.Field)
				text := strings.TrimSpace(in.Text)
				if field == "" || text == "" {
					return `{"ok":false,"error":"field 与 text 不能为空"}`, nil
				}
				p, err := deps.Proposals.Enqueue(ctx, scope, memory.ProposalWrite{
					PersonID: strings.TrimSpace(in.Person), SessionID: sessionID,
					Hypothesis: memory.Hypothesis(strings.TrimSpace(in.Hypothesis)),
					Field:      field, SuggestedText: text, Mode: strings.TrimSpace(in.Mode),
					Rationale: strings.TrimSpace(in.Rationale), Source: "tool",
				})
				if err != nil {
					out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
					return string(out), nil
				}
				out, err := json.Marshal(map[string]any{"ok": true, "proposal": p})
				return string(out), err
			},
		)
		if err != nil {
			return nil, err
		}
		toolsOut = append(toolsOut, proposeBond)
	}

	if !hasGraph {
		return toolsOut, nil
	}

	recallBond, err := utils.InferTool(
		"recall_bond",
		"读取与某人的 Bond 常模。",
		func(ctx context.Context, in personInput) (string, error) {
			bond, err := g.GetBond(ctx, scope, strings.TrimSpace(in.Person))
			if err != nil {
				return "", err
			}
			out, err := json.Marshal(map[string]any{"ok": true, "bond": bond, "text": bond.FormatRecall()})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, recallBond)

	setFact, err := utils.InferTool(
		"set_explicit_bond_fact",
		"仅记录对方明确说出的低风险事实：call_name（对方如何称呼你）或 basics_fact（短句写入 basics）。不可改风格/边界/信任/策略。",
		func(ctx context.Context, in explicitFactInput) (string, error) {
			kind := strings.ToLower(strings.TrimSpace(in.Kind))
			value := strings.TrimSpace(in.Value)
			if value == "" {
				return `{"ok":false,"error":"value 不能为空"}`, nil
			}
			person := strings.TrimSpace(in.Person)
			switch kind {
			case "call_name", "calls":
				if err := g.UpsertCalls(ctx, scope, person, value, strings.TrimSpace(in.SourceEpisodeID)); err != nil {
					return "", err
				}
				out, err := json.Marshal(map[string]any{"ok": true, "kind": "call_name", "value": value})
				return string(out), err
			case "basics_fact", "basics":
				cur, err := g.GetBond(ctx, scope, person)
				if err != nil {
					return "", err
				}
				merged := strings.TrimSpace(cur.Basics)
				if merged != "" && !strings.Contains(merged, value) {
					merged = merged + "\n" + value
				} else if merged == "" {
					merged = value
				}
				bond, err := g.PatchBond(ctx, scope, person, graph.BondPatch{
					Basics: merged, SourceEpisodeID: strings.TrimSpace(in.SourceEpisodeID),
				})
				if err != nil {
					out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
					return string(out), nil
				}
				out, err := json.Marshal(map[string]any{"ok": true, "kind": "basics_fact", "bond": bond})
				return string(out), err
			default:
				return `{"ok":false,"error":"kind 仅支持 call_name|basics_fact"}`, nil
			}
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, setFact)

	return toolsOut, nil
}

type toolHelpInput struct {
	Name string `json:"name" jsonschema:"description=工具名，如 write_episode"`
}

type writeEpisodeInput struct {
	Kind           string `json:"kind" jsonschema:"description=event|preference|boundary|state_observation|self_note；关于自己用 self_note"`
	Content        string `json:"content"`
	Why            string `json:"why,omitempty"`
	About          string `json:"about,omitempty" jsonschema:"description=关于谁：省略时 event 默认当前对话者；self/me/myself 或 kind=self_note 则挂到 Self"`
	ExperienceMode string `json:"experience_mode,omitempty" jsonschema:"description=real_interaction|simulated_roleplay|story_reading|external_observation|self_reflection；缺省 real_interaction；故事/角色代入必须显式标注"`
}

type readEpisodeInput struct {
	ID string `json:"id"`
}

type searchEpisodeInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type statusEpisodeInput struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty" jsonschema:"description=invalidate 时必填"`
}

type personInput struct {
	Person string `json:"person,omitempty"`
}

type proposeBondInput struct {
	Person     string `json:"person,omitempty"`
	Field      string `json:"field" jsonschema:"description=basics|concerns|baseline|strategy|style|boundaries"`
	Text       string `json:"text"`
	Mode       string `json:"mode,omitempty"`
	Hypothesis string `json:"hypothesis,omitempty"`
	Rationale  string `json:"rationale,omitempty"`
}

type explicitFactInput struct {
	Person          string `json:"person,omitempty"`
	Kind            string `json:"kind" jsonschema:"description=call_name|basics_fact"`
	Value           string `json:"value"`
	SourceEpisodeID string `json:"source_episode_id,omitempty"`
}
