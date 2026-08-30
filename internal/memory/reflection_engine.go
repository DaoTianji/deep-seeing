package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
)

type ReflectionCompleter interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

type ReflectionGraph interface {
	GetBond(ctx context.Context, scope identity.TenantScope, personID string) (graph.Bond, error)
}

// ReflectionEngine performs evidence-first T3 consolidation. It deliberately
// uses staged model calls so candidate discovery, reading and evidence use are
// externally observable without exposing hidden reasoning.
type ReflectionEngine struct {
	Chat        ReflectionCompleter
	Store       *ReflectionStore
	Episodes    *EpisodeStore
	Proposals   *ProposalStore
	Dreamer     *Dreamer
	Graph       ReflectionGraph
	Live        *ReflectionLiveStore
	Ledger      *MutationLedger
	Context     ReflectionContextReader
	Tensions    ReflectionTensionResolver
	Mode        ReflectionMode
	Model       string
	SearchLimit int
}

type reflectionPlanOut struct {
	WorthReflecting bool     `json:"worth_reflecting"`
	SeedIDs         []string `json:"seed_ids"`
	Queries         []string `json:"queries"`
	Notes           string   `json:"notes"`
}

type reflectionReadOut struct {
	ReadIDs []string `json:"read_ids"`
	Notes   string   `json:"notes"`
}

type reflectionFinalOut struct {
	NoChange bool   `json:"no_change"`
	Notes    string `json:"notes"`
	Evidence []struct {
		SeedID     string `json:"seed_id"`
		EpisodeID  string `json:"episode_id"`
		State      string `json:"state"`
		ReasonCode string `json:"reason_code"`
	} `json:"evidence"`
	Decisions []struct {
		SeedID            string `json:"seed_id"`
		Action            string `json:"action"`
		CurrentCorrection bool   `json:"current_correction"`
		Kind              string `json:"kind"`
		Field             string `json:"field"`
		SuggestedText     string `json:"suggested_text"`
		Mode              string `json:"mode"`
		Reason            string `json:"reason_summary"`
	} `json:"decisions"`
}

type reflectionCandidate struct {
	ID             string         `json:"id"`
	Kind           EpisodeKind    `json:"kind"`
	Summary        string         `json:"summary"`
	ExperienceMode ExperienceMode `json:"experience_mode"`
	CreatedAt      time.Time      `json:"created_at"`
}

type reflectionReadEvidence struct {
	ID             string            `json:"id"`
	Kind           EpisodeKind       `json:"kind"`
	Content        string            `json:"content"`
	Why            string            `json:"why,omitempty"`
	ExperienceMode ExperienceMode    `json:"experience_mode"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

func (e *ReflectionEngine) Run(ctx context.Context, scope identity.TenantScope, sessionID string, trigger ReflectionTrigger) (ReflectionRun, error) {
	started := time.Now().UTC()
	mode := e.Mode
	if mode == "" {
		mode = ReflectionModeObserve
	}
	run := ReflectionRun{
		ID: "rrun_" + strings.ReplaceAll(uuid.NewString(), "-", ""), PersonID: scope.PersonID(),
		SessionID: sessionID, Mode: mode, Trigger: trigger, StartedAt: started,
	}
	e.Live.Update(run, "starting", true)
	consumedDirty := false
	finish := func(err error) (ReflectionRun, error) {
		run.CompletedAt = time.Now().UTC()
		run.Duration = run.CompletedAt.Sub(run.StartedAt)
		if err != nil {
			run.Error = truncate(err.Error(), 240)
		}
		if e.Store != nil {
			if saved, appendErr := e.Store.AppendRun(run); appendErr == nil {
				run = saved
			} else if err == nil {
				err = appendErr
			}
		}
		e.Live.Update(run, "completed", false)
		if err == nil && consumedDirty && e.Store != nil && mode != ReflectionModeLegacy {
			_ = e.Store.ClearDirty(scope.PersonID())
		}
		return run, err
	}
	if err := scope.Validate(); err != nil {
		return finish(err)
	}
	if e.Store == nil || e.Episodes == nil || e.Proposals == nil {
		run.NoChange, run.Notes = true, "Reflection pipeline unavailable."
		return finish(nil)
	}
	if mode == ReflectionModeLegacy {
		if e.Dreamer == nil {
			run.NoChange, run.Notes = true, "Legacy Dream unavailable."
			return finish(nil)
		}
		legacy, err := e.Dreamer.Run(ctx, scope, trigger == ReflectionTriggerManual)
		run.NoChange, run.Notes = legacy.Skipped || len(legacy.MutationIDs) == 0, legacy.Notes
		run.MutationIDs = append(run.MutationIDs, legacy.MutationIDs...)
		return finish(err)
	}
	if e.Chat == nil {
		run.NoChange, run.Notes = true, "No reflection model; No change."
		return finish(nil)
	}
	seeds, err := e.Store.List(ctx, scope, scope.PersonID(), false, 50)
	if err != nil {
		return finish(err)
	}
	now := time.Now().UTC()
	eligible := seeds[:0]
	for _, seed := range seeds {
		if seed.Status != ReflectionSeedOpen && seed.Status != ReflectionSeedDeferred {
			continue
		}
		if !seed.NextReviewAt.IsZero() && now.Before(seed.NextReviewAt) && trigger != ReflectionTriggerManual {
			continue
		}
		eligible = append(eligible, seed)
	}
	if len(eligible) == 0 {
		run.NoChange, run.Notes = true, "No open reflection seeds."
		return finish(nil)
	}
	consumedDirty = true

	bond := graph.Bond{PersonID: scope.PersonID()}
	if e.Graph != nil {
		if loaded, loadErr := e.Graph.GetBond(ctx, scope, scope.PersonID()); loadErr == nil {
			bond = loaded
		}
	}
	selfContext := loadReflectionContext(ctx, e.Context, scope)
	plan, err := e.plan(ctx, eligible, bond, selfContext)
	if err != nil {
		return finish(err)
	}
	run.Notes = strings.TrimSpace(plan.Notes)
	if !plan.WorthReflecting {
		run.NoChange = true
		if run.Notes == "" {
			run.Notes = "No change."
		}
		return finish(nil)
	}
	seedMap := make(map[string]ReflectionSeed, len(eligible))
	for _, seed := range eligible {
		seedMap[seed.ID] = seed
	}
	selected := selectKnownSeeds(plan.SeedIDs, eligible)
	if len(selected) == 0 {
		selected = eligible[:1]
	}
	for _, seed := range selected {
		run.SeedIDs = append(run.SeedIDs, seed.ID)
	}
	queries := uniqueStrings(plan.Queries)
	if len(queries) == 0 {
		for _, seed := range selected {
			queries = append(queries, seed.Statement)
		}
	}
	if len(queries) > 4 {
		queries = queries[:4]
	}
	run.Queries = append(run.Queries, queries...)
	e.Live.Update(run, "selecting", true)
	candidateMap := map[string]Episode{}
	limit := e.SearchLimit
	if limit <= 0 {
		limit = 8
	}
	for _, query := range queries {
		episodes, searchErr := e.Episodes.Search(ctx, scope, Query{Text: query, Limit: limit, IncludeRole: true})
		if searchErr != nil {
			return finish(searchErr)
		}
		for _, ep := range episodes {
			candidateMap[ep.ID] = ep
		}
	}
	var cards []reflectionCandidate
	for _, ep := range candidateMap {
		run.CandidateIDs = append(run.CandidateIDs, ep.ID)
		cards = append(cards, reflectionCandidate{
			ID: ep.ID, Kind: ep.Kind, Summary: graph.SummaryFromContent(ep.Content, 160),
			ExperienceMode: ep.ExperienceMode, CreatedAt: ep.CreatedAt,
		})
	}
	run.CandidateIDs = uniqueStrings(run.CandidateIDs)
	e.Live.Update(run, "candidates", true)
	readPlan, err := e.chooseReads(ctx, selected, cards)
	if err != nil {
		return finish(err)
	}
	readMap := map[string]Episode{}
	for _, id := range uniqueStrings(readPlan.ReadIDs) {
		if _, candidate := candidateMap[id]; !candidate {
			continue
		}
		ep, readErr := e.Episodes.Get(ctx, id)
		if readErr != nil {
			continue
		}
		readMap[id] = ep
		run.ReadIDs = append(run.ReadIDs, id)
	}
	e.Live.Update(run, "reading", true)
	final, err := e.decide(ctx, selected, bond, selfContext, readMap)
	if err != nil {
		return finish(err)
	}
	run.NoChange = final.NoChange
	if strings.TrimSpace(final.Notes) != "" {
		run.Notes = strings.TrimSpace(final.Notes)
	}
	run.Evidence = validateReflectionEvidence(final, selected, candidateMap, readMap)
	e.Live.Update(run, "evidence", true)
	run.Decisions = e.applyDecisions(ctx, scope, &run, final, seedMap, bond)
	run.NoChange = !hasConsolidatingDecision(run.Decisions)
	for _, decision := range run.Decisions {
		if decision.MutationID != "" {
			run.MutationIDs = append(run.MutationIDs, decision.MutationID)
		}
	}
	e.Live.Update(run, "consolidating", true)
	return finish(nil)
}

func hasConsolidatingDecision(decisions []ReflectionDecision) bool {
	for _, decision := range decisions {
		switch decision.Action {
		case ReflectionConfirm, ReflectionRevise, ReflectionSupersede, ReflectionOpenTension, ReflectionResolveTension:
			return true
		}
	}
	return false
}

func (e *ReflectionEngine) plan(ctx context.Context, seeds []ReflectionSeed, bond graph.Bond, selfContext []ReflectionContextItem) (reflectionPlanOut, error) {
	system := strings.TrimSpace(`
用户当前的直接纠正若会修订旧的长期认识，仍值得进入本路径，并应搜索可能被纠正的旧证据。direct_expression 中明确新增或加强的私人边界，除非明说只限一次，否则应视为持续有效的 Bond 边界：必须选择并搜索旧相关经历，不能仅以“当前情境”跳过。
若 Seed 来自 model_inference 且命题一旦成立会改变 Bond，必须先搜索相关 Episode 核验来源；即使预期结论是不修改，也不能在 plan 阶段跳过。若问题是在检查 roleplay/story/推断/generated 是否污染了真实认识，应调查相关 Episode 并明确来源隔离。
Episode 的来源性质以 experience_mode 和 epistemic 元数据为准，不要只凭正文里出现“虚构”“故事”等词判断。
你在决定一次证据型反思是否值得发生。选择真正需要跨经历核验的 ReflectionSeed，并给出自然语言记忆搜索查询。Seed 不能独立证明外部事实，但可以可信地描述这个反思问题的流程状态；若命题明确说明误会已当场澄清、问题已解决或无需重谈，且没有新的冲突，应 worth_reflecting=false，不要因缺少 Episode 而 defer。若 Seed 命题本身询问跨经历的稳定、重复或冲突模式，且不是明确锚定某一次普通任务或临时状态，应先搜索再判断，不要假设没有历史证据。
普通、已解决、仅凭一次情绪推断的问题可以不选。若 Seed 只把一次普通任务（如答题、润色）上升为稳定 Self Pattern，且没有指向其他 Episode，应直接 worth_reflecting=false 与 no_change，而不是 defer。不要因为系统给了 Seed 就强行制造结论。
最多选择少量最重要问题。对需要跨经历核验的问题，通常给 2–4 条短而互补的搜索表达，分别覆盖命题原词、可观察行为、可能原因或反例；不要把来源排除条件和完整分析写成一条长查询，来源隔离由元数据判断。查询仍由你按语义自主决定，不是固定关键词规则。只输出 JSON。
{"worth_reflecting":false,"seed_ids":[],"queries":[],"notes":"No change."}`)
	payload := map[string]any{"bond_version": bond.Version, "bond": bond.FormatRecall(), "self_patterns_and_tensions": selfContext, "reflection_seeds": seedCards(seeds)}
	raw, err := e.Chat.Complete(ctx, system, mustJSON(payload))
	if err != nil {
		return reflectionPlanOut{}, err
	}
	var out reflectionPlanOut
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &out); err != nil {
		return out, fmt.Errorf("parse reflection plan: %w", err)
	}
	return out, nil
}

func (e *ReflectionEngine) chooseReads(ctx context.Context, seeds []ReflectionSeed, candidates []reflectionCandidate) (reflectionReadOut, error) {
	if len(candidates) == 0 {
		return reflectionReadOut{Notes: "No candidates."}, nil
	}
	system := strings.TrimSpace(`
你在挑选证据型反思需要真正阅读全文的 Episode。候选卡只有摘要，不能作为事实依据。
选择可能支持、反驳或提供必要上下文的候选；无关候选不要读取。若反思问题是在核验来源污染，相关 roleplay/story/simulation 候选也必须读取至少一条，候选卡上的标签不足以完成证据声明。只输出 JSON。
{"read_ids":["ep_..."],"notes":"简短公开说明"}`)
	payload := map[string]any{"reflection_seeds": seedCards(seeds), "candidate_cards": candidates}
	raw, err := e.Chat.Complete(ctx, system, mustJSON(payload))
	if err != nil {
		return reflectionReadOut{}, err
	}
	var out reflectionReadOut
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &out); err != nil {
		return out, fmt.Errorf("parse reflection reads: %w", err)
	}
	return out, nil
}

func (e *ReflectionEngine) decide(ctx context.Context, seeds []ReflectionSeed, bond graph.Bond, selfContext []ReflectionContextItem, read map[string]Episode) (reflectionFinalOut, error) {
	var evidence []reflectionReadEvidence
	for _, ep := range read {
		evidence = append(evidence, reflectionReadEvidence{
			ID: ep.ID, Kind: ep.Kind, Content: ep.Content, Why: ep.Why,
			ExperienceMode: ep.ExperienceMode, Metadata: ep.Metadata, CreatedAt: ep.CreatedAt,
		})
	}
	system := strings.TrimSpace(`
你在进行证据型反思。Episode 正文是可核验历史；ReflectionSeed、Session Review 状态观察和 Proposal 都不是独立事实。

规则：
1. 为实际使用的 Episode 声明 support|conflict|context；排除项可声明 duplicate|stale|insufficient|superseded。
2. Evidence state 描述单条 Episode 与 Seed 命题的局部关系，不等于最终结论是否充分：真实事件直接呈现命题一侧时标 support 或 context，不能仅因样本少就把该事件标 insufficient；证据总量不足应在 Decision 选择 defer。
3. 用户当前明确表达优先于旧 Episode；一次临时状态不仅不能泛化为稳定人格，也不能自动写成未来的条件策略（例如一次疲惫时要求简短，不等于以后疲惫时都应简短）。只能保留为“当时”的历史背景，未来应以当下表达为准。若已读 Episode 彼此相反，分别标 conflict 或 context，不要因为它们共同证明“存在张力”就都标 support。
4. roleplay/story/simulation 不得升级为真实关系事实；生成假设不能证明自己。
5. action 仅可为 no_change|defer|confirm|revise|supersede|open_tension|resolve_tension|reject_seed。
6. kind 仅可为 bond|self_pattern|tension|principle；principle 只能形成候选，Soul/World 不可写。
7. revise/supersede/open_tension 必须给 field、suggested_text 和简短 reason_summary。Bond 的 field 必须是 basics|interaction|boundaries|priorities|baseline；回答详略、沟通偏好归 interaction。
8. 有真实正例与反例时用 defer、open_tension 或有限 revise；reject_seed 只用于来源无效、问题确属误判或已无意义，不能用来消除仍存在的冲突。允许每个 Seed 独立没有变化。只输出 JSON，不要输出隐藏思考。
9. Episode 的事实层级由 experience_mode 与 epistemic 元数据决定；正文提到“虚构”不自动等于 generated。evaluation_fixture=true 表示它是测试宇宙中的模拟记录，仍按声明的 experience_mode 评价。
10. 当前 direct_expression Seed 可以作为本轮依据。明确新增或加强的私人边界，除非用户说只限一次，否则 action 必须 revise 或 supersede 到 Bond boundaries；旧亲密或开放经历只能是 context|stale|superseded|conflict，不能削弱当前边界。若它在纠正旧认识，decision 必须设置 current_correction=true，action 使用 revise 或 supersede，不能 confirm；相关旧 Episode 标 conflict|context|stale|superseded，不能标 support。
11. 多条独立、直接、真实且一致的 Episode 足以支持范围受限的 interaction 偏好时，应 confirm 或 revise 并收窄措辞；不要仅因它不是跨所有场景的人格规律而 defer。
12. 若 Seed 命题明确说明误会已当场澄清或问题已经解决，且没有读到新的冲突，选择 no_change 或 reject_seed；不得把它 defer 成长期 Tension。
13. 巩固重复模式时只写多条证据共同支持的可观察交集；原因、动机或人格解释若不是每条采用证据都明确给出，就写成未知，不能补成统一因果。若 revise 正在把过宽 Seed 收窄，证据状态应同时依据最终 suggested_text：直接体现修订后窄模式的 Episode 必须标 support；context 只用于提供背景但没有直接体现该模式的经历。

{"no_change":false,"notes":"公开结论","evidence":[{"seed_id":"ref_...","episode_id":"ep_...","state":"support","reason_code":""}],"decisions":[{"seed_id":"ref_...","action":"defer","current_correction":false,"kind":"bond","field":"","suggested_text":"","mode":"append","reason_summary":"证据不足"}]}`)
	payload := map[string]any{"bond_version": bond.Version, "bond": bond.FormatRecall(), "self_patterns_and_tensions": selfContext, "reflection_seeds": seedCards(seeds), "read_episodes": evidence}
	raw, err := e.Chat.Complete(ctx, system, mustJSON(payload))
	if err != nil {
		return reflectionFinalOut{}, err
	}
	var out reflectionFinalOut
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &out); err != nil {
		return out, fmt.Errorf("parse reflection decision: %w", err)
	}
	return out, nil
}

func validateReflectionEvidence(out reflectionFinalOut, seeds []ReflectionSeed, candidates, read map[string]Episode) []ReflectionEvidence {
	seedIDs := map[string]bool{}
	directSeedIDs := map[string]bool{}
	seedScopes := map[string]ReflectionScope{}
	for _, seed := range seeds {
		seedIDs[seed.ID] = true
		directSeedIDs[seed.ID] = seed.SourceType == ReflectionSourceDirect
		seedScopes[seed.ID] = seed.Scope
	}
	currentCorrections := map[string]bool{}
	for _, decision := range out.Decisions {
		if decision.CurrentCorrection {
			currentCorrections[strings.TrimSpace(decision.SeedID)] = true
		}
	}
	var result []ReflectionEvidence
	seen := map[string]bool{}
	for _, item := range out.Evidence {
		id, seedID := strings.TrimSpace(item.EpisodeID), strings.TrimSpace(item.SeedID)
		ep, candidate := candidates[id]
		if !candidate || !seedIDs[seedID] {
			continue
		}
		state := normalizeReflectionEvidenceState(item.State)
		if state == ReflectionEvidenceSupport && (seedScopes[seedID] == ReflectionScopeTension || (currentCorrections[seedID] && directSeedIDs[seedID])) {
			state = ReflectionEvidenceContext
		}
		if state == "" {
			continue
		}
		key := seedID + "|" + id
		if seen[key] {
			continue
		}
		seen[key] = true
		_, wasRead := read[id]
		if evidenceStateNeedsRead(state) && !wasRead {
			continue
		}
		epistemic := "observed"
		if ep.Metadata["epistemic"] == "derived_inference" || ep.Why == "session_review" || ep.ExperienceMode == ExperienceSelfReflection {
			epistemic = "derived_inference"
		}
		result = append(result, ReflectionEvidence{
			SeedID: seedID, EpisodeID: id, State: state, ReasonCode: normalizeEvidenceReason(item.ReasonCode),
			Read: wasRead, ExperienceMode: ep.ExperienceMode, Epistemic: epistemic,
		})
	}
	return result
}

func (e *ReflectionEngine) applyDecisions(ctx context.Context, scope identity.TenantScope, run *ReflectionRun, out reflectionFinalOut, seeds map[string]ReflectionSeed, bond graph.Bond) []ReflectionDecision {
	var decisions []ReflectionDecision
	seen := map[string]bool{}
	for _, item := range out.Decisions {
		seed, ok := seeds[strings.TrimSpace(item.SeedID)]
		if !ok || seen[seed.ID] {
			continue
		}
		seen[seed.ID] = true
		kind, validKind := normalizeReflectionDecisionKind(item.Kind)
		decision := ReflectionDecision{
			SeedID: seed.ID, Action: normalizeReflectionAction(item.Action), CurrentCorrection: item.CurrentCorrection, Kind: kind,
			Field: strings.TrimSpace(item.Field), SuggestedText: strings.TrimSpace(item.SuggestedText),
			Mode: strings.TrimSpace(item.Mode), ReasonSummary: truncate(item.Reason, 240), ExpectedBondVersion: bond.Version,
		}
		if decision.Action == "" {
			decision.Action = ReflectionDefer
		}
		if !validKind {
			decision.Action = ReflectionDefer
			decision.ReasonSummary = "reflection target is outside T3 write scope"
		}
		if decision.CurrentCorrection && decision.Action == ReflectionConfirm {
			decision.Action = ReflectionRevise
		}
		if decision.Kind == ProposalKindBond {
			decision.Field = normalizeProposalField(decision.Field)
		}
		used := evidenceForSeed(run.Evidence, seed.ID)
		validEvidence := seed.SourceType == ReflectionSourceDirect && !seed.Generated
		for _, ev := range used {
			if reflectionEvidenceCanSupport(decision.Kind, ev) {
				validEvidence = true
			}
		}
		mutating := decision.Action == ReflectionRevise || decision.Action == ReflectionSupersede || decision.Action == ReflectionOpenTension || decision.Action == ReflectionResolveTension
		if decision.Action == ReflectionConfirm && !validEvidence {
			decision.Action = ReflectionDefer
			decision.ReasonSummary = firstNonEmpty(decision.ReasonSummary, "confirmation requires verified evidence")
		}
		if mutating && (!validEvidence || strings.TrimSpace(decision.SuggestedText) == "") {
			decision.Action = ReflectionDefer
			decision.ReasonSummary = firstNonEmpty(decision.ReasonSummary, "insufficient verified evidence")
		}
		if decision.Action == ReflectionOpenTension {
			decision.Kind = ProposalKindTension
		}
		if decision.Kind == ProposalKindPrinciple && mutating {
			decision.Action = ReflectionDefer
			decision.ReasonSummary = firstNonEmpty(decision.ReasonSummary, "principle remains a candidate in T3")
		}

		switch decision.Action {
		case ReflectionRejectSeed:
			_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedRejected, time.Time{})
		case ReflectionNoChange:
			_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedResolved, time.Time{})
		case ReflectionConfirm:
			if e.Mode == ReflectionModeAgent && e.Ledger != nil {
				confirmed, confirmErr := e.Ledger.Append(Mutation{
					Kind: "reflection_confirm", PersonID: seed.PersonID,
					SourceEpisodeIDs: evidenceEpisodeIDs(used), ReflectionSeedID: seed.ID,
					ReflectionRunID: run.ID, Actor: "reflection", ModelVersion: e.Model,
					ReasonSummary: decision.ReasonSummary,
				})
				if confirmErr != nil {
					decision.Action = ReflectionDefer
					decision.ReasonSummary = truncate(confirmErr.Error(), 240)
					_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedDeferred, time.Now().UTC().Add(24*time.Hour))
					break
				}
				decision.MutationID = confirmed.ID
			}
			_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedResolved, time.Time{})
		case ReflectionDefer:
			_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedDeferred, time.Now().UTC().Add(24*time.Hour))
		case ReflectionResolveTension:
			if e.Mode == ReflectionModeAgent && e.Tensions != nil && e.Ledger != nil {
				mutationID, resolveErr := e.resolveTension(ctx, scope, run.ID, seed, decision, used)
				if resolveErr != nil {
					decision.Action = ReflectionDefer
					decision.ReasonSummary = truncate(resolveErr.Error(), 240)
					_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedDeferred, time.Now().UTC().Add(24*time.Hour))
					break
				}
				decision.MutationID = mutationID
				_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedResolved, time.Time{})
				break
			}
			fallthrough
		case ReflectionRevise, ReflectionSupersede, ReflectionOpenTension:
			proposal, err := e.Proposals.Enqueue(ctx, scope, ProposalWrite{
				Kind: decision.Kind, PersonID: seed.PersonID, SessionID: seed.SessionID,
				Hypothesis: HypothesisH1, Field: decision.Field, SuggestedText: decision.SuggestedText,
				Mode: decision.Mode, Rationale: decision.ReasonSummary, Source: "reflection",
				ExperienceModes: evidenceModes(used), SourceEpisodeIDs: evidenceEpisodeIDs(used),
				ReflectionSeedID: seed.ID, ReflectionRunID: run.ID, ExpectedVersion: bond.Version,
			})
			if err != nil {
				decision.Action = ReflectionDefer
				decision.ReasonSummary = truncate(err.Error(), 240)
				_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedDeferred, time.Now().UTC().Add(24*time.Hour))
				break
			}
			decision.ProposalID = proposal.ID
			if e.Mode == ReflectionModeAgent && e.Dreamer != nil && decision.Kind != ProposalKindPrinciple && decision.Action != ReflectionResolveTension {
				accepted, acceptErr := e.Dreamer.ApplyAccept(ctx, scope, proposal.ID, run.ID, decision.ReasonSummary)
				if acceptErr == nil && len(accepted.MutationIDs) > 0 {
					decision.MutationID = accepted.MutationIDs[0]
					_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedResolved, time.Time{})
				} else {
					_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedDeferred, time.Now().UTC().Add(24*time.Hour))
					if acceptErr != nil {
						decision.ReasonSummary = truncate(acceptErr.Error(), 240)
					}
				}
			} else {
				_, _ = e.Store.UpdateStatus(ctx, seed.ID, ReflectionSeedDeferred, time.Now().UTC().Add(24*time.Hour))
			}
		}
		decisions = append(decisions, decision)
	}
	return decisions
}

func seedCards(seeds []ReflectionSeed) []map[string]any {
	out := make([]map[string]any, 0, len(seeds))
	for _, seed := range seeds {
		out = append(out, map[string]any{
			"id": seed.ID, "scope": seed.Scope, "statement": seed.Statement,
			"source_type": seed.SourceType, "generated": seed.Generated,
			"experience_modes": seed.ExperienceModes, "created_at": seed.CreatedAt,
		})
	}
	return out
}

func selectKnownSeeds(ids []string, seeds []ReflectionSeed) []ReflectionSeed {
	byID := map[string]ReflectionSeed{}
	for _, seed := range seeds {
		byID[seed.ID] = seed
	}
	var out []ReflectionSeed
	for _, id := range uniqueStrings(ids) {
		if seed, ok := byID[id]; ok {
			out = append(out, seed)
		}
	}
	return out
}

func normalizeReflectionEvidenceState(raw string) ReflectionEvidenceState {
	switch ReflectionEvidenceState(strings.ToLower(strings.TrimSpace(raw))) {
	case ReflectionEvidenceSupport, ReflectionEvidenceConflict, ReflectionEvidenceContext,
		ReflectionEvidenceDuplicate, ReflectionEvidenceStale, ReflectionEvidenceInsufficient, ReflectionEvidenceSuperseded:
		return ReflectionEvidenceState(strings.ToLower(strings.TrimSpace(raw)))
	default:
		return ""
	}
}

func normalizeReflectionDecisionKind(raw string) (ProposalKind, bool) {
	switch ProposalKind(strings.ToLower(strings.TrimSpace(raw))) {
	case ProposalKindBond, ProposalKindSelfPattern, ProposalKindTension, ProposalKindPrinciple:
		return ProposalKind(strings.ToLower(strings.TrimSpace(raw))), true
	default:
		return "", false
	}
}

func normalizeReflectionAction(raw string) ReflectionAction {
	switch ReflectionAction(strings.ToLower(strings.TrimSpace(raw))) {
	case ReflectionNoChange, ReflectionDefer, ReflectionConfirm, ReflectionRevise, ReflectionSupersede,
		ReflectionOpenTension, ReflectionResolveTension, ReflectionRejectSeed:
		return ReflectionAction(strings.ToLower(strings.TrimSpace(raw)))
	default:
		return ""
	}
}

func evidenceStateNeedsRead(state ReflectionEvidenceState) bool {
	return state != ""
}

func normalizeEvidenceReason(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "irrelevant", "conflicting", "stale", "insufficient", "superseded", "duplicate":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func evidenceForSeed(all []ReflectionEvidence, seedID string) []ReflectionEvidence {
	var out []ReflectionEvidence
	for _, evidence := range all {
		if evidence.SeedID == seedID {
			out = append(out, evidence)
		}
	}
	return out
}

func reflectionEvidenceCanSupport(kind ProposalKind, ev ReflectionEvidence) bool {
	if !ev.Read || ev.Epistemic != "observed" {
		return false
	}
	if ev.State != ReflectionEvidenceSupport && ev.State != ReflectionEvidenceConflict && ev.State != ReflectionEvidenceContext {
		return false
	}
	switch NormalizeProposalKind(string(kind)) {
	case ProposalKindBond:
		return NormalizeExperienceMode(string(ev.ExperienceMode)) == ExperienceRealInteraction
	case ProposalKindPrinciple:
		return false
	default:
		return true
	}
}

func evidenceEpisodeIDs(all []ReflectionEvidence) []string {
	var ids []string
	for _, ev := range all {
		if ev.Read && ev.Epistemic == "observed" && (ev.State == ReflectionEvidenceSupport || ev.State == ReflectionEvidenceConflict || ev.State == ReflectionEvidenceContext) {
			ids = append(ids, ev.EpisodeID)
		}
	}
	return uniqueStrings(ids)
}

func evidenceModes(all []ReflectionEvidence) []ExperienceMode {
	var modes []ExperienceMode
	for _, ev := range all {
		if ev.Read {
			modes = append(modes, ev.ExperienceMode)
		}
	}
	return normalizeExperienceModes(modes)
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
