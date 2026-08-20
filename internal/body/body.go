package body

import (
	"os"
	"strings"
	"time"

	"deep-seeing/internal/identity"
)

// Versions are bumpable contracts for birth / observability.
const (
	SoulVersion        = "0.2"
	OriginVersion      = "0.1"
	ToolsetVersion     = "1.0"
	GraphSchemaVersion = "0.3"
	CapabilityCatalogV = "1.0"
)

// Snapshot is what inspect_runtime returns — existence facts, not philosophy.
type Snapshot struct {
	AgentID       string            `json:"agent_id"`
	CurrentPerson string            `json:"current_person"`
	SessionID     string            `json:"session_id"`
	Now           string            `json:"now"`
	Timezone      string            `json:"timezone"`
	Model         string            `json:"model"`
	Versions      map[string]string `json:"versions"`
	Stores        map[string]string `json:"stores"`
	Persistence   map[string]string `json:"persistence"`
	FirstBoot     bool              `json:"first_boot_origin,omitempty"`
}

// BuildSnapshot assembles runtime identity for the agent.
func BuildSnapshot(scope identity.TenantScope, sessionID, model string, stores map[string]string, firstBoot bool) Snapshot {
	tz := "Asia/Shanghai"
	if v := strings.TrimSpace(os.Getenv("TZ")); v != "" {
		tz = v
	}
	loc, err := time.LoadLocation(tz)
	now := time.Now()
	if err == nil {
		now = now.In(loc)
	}
	if stores == nil {
		stores = map[string]string{}
	}
	return Snapshot{
		AgentID:       scope.AgentID,
		CurrentPerson: scope.PersonID(),
		SessionID:     sessionID,
		Now:           now.Format(time.RFC3339),
		Timezone:      tz,
		Model:         model,
		Versions: map[string]string{
			"soul":           SoulVersion,
			"origin":         OriginVersion,
			"toolset":        ToolsetVersion,
			"graph_schema":   GraphSchemaVersion,
			"capability_cat": CapabilityCatalogV,
		},
		Stores: stores,
		Persistence: map[string]string{
			"stm":       "temporary",
			"episode":   "persistent",
			"graph":     "persistent",
			"proposal":  "persistent",
			"self":      "persistent",
			"workspace": "persistent",
			"intent":    "persistent",
			"source":    "persistent",
			"scene":     "persistent",
		},
		FirstBoot: firstBoot,
	}
}

// Capability describes one tool for list_capabilities / tool_help.
type Capability struct {
	Name        string `json:"name"`
	Ability     string `json:"ability"`
	Persistence string `json:"persistence"` // none | session | cross-session | delayed
	SideEffect  string `json:"side_effect"`
	Permission  string `json:"permission"` // observe | internal | external
	Help        string `json:"help"`
}

// Catalog is the thin capability table (not dumped into system prompt).
func Catalog(hasGraph, hasProposals, hasSelf, hasWorkspace, hasIntents, hasWorld, hasScenes bool) []Capability {
	out := []Capability{
		{Name: "inspect_runtime", Ability: "查看当前身体/版本/持久性", Persistence: "none", SideEffect: "只读", Permission: "observe",
			Help: "返回 agent_id、当前对话者、时间、模型与各存储是否可用。"},
		{Name: "list_capabilities", Ability: "列出可用工具摘要", Persistence: "none", SideEffect: "只读", Permission: "observe",
			Help: "返回能力表；不要依赖 System Prompt 里的完整工具清单。"},
		{Name: "tool_help", Ability: "查询单个工具说明", Persistence: "none", SideEffect: "只读", Permission: "observe",
			Help: "传入 tool_name，返回用途与副作用。"},
		{Name: "get_time", Ability: "查看当前时间", Persistence: "none", SideEffect: "只读", Permission: "observe",
			Help: "返回 RFC3339 时间与时区。"},
		{Name: "write_episode", Ability: "记住经历（含关于自己）", Persistence: "cross-session", SideEffect: "写 LTM", Permission: "internal",
			Help: "对人的事默认 about 当前对话者；对自己的理解用 kind=self_note 或 about=self。故事/角色代入必须设 experience_mode=simulated_roleplay|story_reading（缺省 real_interaction）。"},
		{Name: "read_episode", Ability: "读取一条经历", Persistence: "none", SideEffect: "只读", Permission: "observe",
			Help: "按 id 读正文，含已归档/失效条目。"},
		{Name: "search_episodes", Ability: "搜索过去经历", Persistence: "none", SideEffect: "只读", Permission: "observe",
			Help: "字面检索；默认不含 archived/invalid。"},
		{Name: "archive_episode", Ability: "归档经历（软忘记）", Persistence: "cross-session", SideEffect: "降权，不物理删除", Permission: "internal",
			Help: "status=archived；召回默认跳过。"},
		{Name: "invalidate_episode", Ability: "判定经历无效", Persistence: "cross-session", SideEffect: "降权+原因", Permission: "internal",
			Help: "status=invalid；图侧可降 confidence。"},
	}
	if hasWorkspace {
		out = append(out,
			Capability{Name: "list_workspace", Ability: "列出未完成思考", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "question/writing/research/project；与 SelfArtifact 分立。"},
			Capability{Name: "read_workspace", Ability: "读取 Workspace 文档", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "含正文、修订史、关联 Episode。"},
			Capability{Name: "write_workspace", Ability: "创建或续写 Workspace", Persistence: "cross-session", SideEffect: "写文件", Permission: "internal",
				Help: "未完成思考草稿；更新追加 revision。不是 Self 结晶。"},
			Capability{Name: "link_workspace_episode", Ability: "关联 Episode 到 Workspace", Persistence: "cross-session", SideEffect: "写链接", Permission: "internal",
				Help: "episode_id 去重追加。"},
		)
	}
	if hasIntents {
		out = append(out,
			Capability{Name: "list_intents", Ability: "列出未来 Intent", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "活跃的 one_shot/recurring 唤醒约定。"},
			Capability{Name: "read_intent", Ability: "读取 Intent 与 wake 历史", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "含 catch-up 后的 wake_jobs。"},
			Capability{Name: "create_intent", Ability: "留下未来 Intent", Persistence: "cross-session", SideEffect: "写 runtime.db", Permission: "internal",
				Help: "默认不主动联系人；周期 Intent 需 interval。"},
			Capability{Name: "cancel_intent", Ability: "取消 Intent", Persistence: "cross-session", SideEffect: "改状态", Permission: "internal",
				Help: "status=cancelled。"},
		)
	}
	if hasWorld {
		out = append(out,
			Capability{Name: "search_web", Ability: "搜索公开网页", Persistence: "none", SideEffect: "外网请求", Permission: "external",
				Help: "经 World Gateway；结果不可信，须 fence。受日预算限制。"},
			Capability{Name: "read_webpage", Ability: "抓取网页正文", Persistence: "cross-session", SideEffect: "外网+可落盘 Source", Permission: "external",
				Help: "SSRF 防护；正文 UNTRUSTED_EXTERNAL_CONTENT。可存 sources/。"},
			Capability{Name: "list_sources", Ability: "列出已存外部资料", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "本地 Source 索引，不含全文。"},
			Capability{Name: "read_source", Ability: "读取已存 Source", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "按 id 读摘要/正文；仍属不可信外部内容。"},
		)
	}
	if hasSelf {
		out = append(out,
			Capability{Name: "inspect_self", Ability: "观察自我理解", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "返回 SelfArtifact 概览或按 type 过滤列表。"},
			Capability{Name: "trace_self_belief", Ability: "追溯自我主张证据", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "按 id 读正文、修订史、experience_mode 与 SUPPORTED_BY/CHALLENGED_BY。"},
			Capability{Name: "list_self_tensions", Ability: "列出开放张力", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "列出非 deprecated 的 tensions。"},
		)
		if hasProposals {
			out = append(out, Capability{Name: "propose_self_update", Ability: "提议自我理解更新", Persistence: "delayed", SideEffect: "入提案队列，慢变", Permission: "internal",
				Help: "kind=self_pattern|principle|tension；Dream 才可能采纳。Soul 永不入队。roleplay-only 不得升 principle。"})
		}
	}
	if hasProposals {
		out = append(out, Capability{Name: "propose_bond_update", Ability: "提议改变长期认识", Persistence: "delayed", SideEffect: "入提案队列，慢变", Permission: "internal",
			Help: "slot+claim 提案：basics|interaction|boundaries|priorities|baseline；不可提案 strategy。Dream 才可能采纳。"})
	}
	if hasScenes {
		out = append(out,
			Capability{Name: "list_scene_norms", Ability: "列出场景常模", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "按人列出 SceneNorm；非全局 Bond。"},
			Capability{Name: "read_scene_norm", Ability: "读取场景常模", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "按 id 读场景常模正文与关键词。"},
			Capability{Name: "write_scene_norm", Ability: "写入场景常模", Persistence: "cross-session", SideEffect: "写本地 SceneNorm", Permission: "internal",
				Help: "须 keywords；仅关键词命中时旁路注入；去掉场景后不应仍当全局真理。"},
		)
	}
	if hasGraph {
		out = append(out,
			Capability{Name: "recall_bond", Ability: "看关系常模", Persistence: "none", SideEffect: "只读", Permission: "observe",
				Help: "读取 Bond compact（Item SoT；Strategy 派生缓存版本匹配时注入）。"},
			Capability{Name: "set_explicit_bond_fact", Ability: "记录明确低风险事实", Persistence: "cross-session", SideEffect: "写 CALLS 或 basics Item", Permission: "internal",
				Help: "仅用于对方明确说出的事实（如称呼）；不可改性格/信任/边界。"},
			Capability{Name: "append_bond_boundary", Ability: "重大错误快写边界", Persistence: "cross-session", SideEffect: "直写 Boundaries Item", Permission: "internal",
				Help: "唯一允许直写的常模槽；他指或自发现的不可再犯结论。"},
			Capability{Name: "set_bond_strategy_cache", Ability: "刷新 Strategy 派生缓存", Persistence: "cross-session", SideEffect: "写 strategy_cache（非 SoT）", Permission: "internal",
				Help: "绑定当前 bond_version；Item 变更后需重刷才会再注入。"},
		)
	}
	return out
}

// FindCapability looks up one tool in the catalog.
func FindCapability(name string, hasGraph, hasProposals, hasSelf, hasWorkspace, hasIntents, hasWorld, hasScenes bool) (Capability, bool) {
	name = strings.TrimSpace(name)
	for _, c := range Catalog(hasGraph, hasProposals, hasSelf, hasWorkspace, hasIntents, hasWorld, hasScenes) {
		if c.Name == name {
			return c, true
		}
	}
	return Capability{}, false
}
