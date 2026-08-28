package runtime

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/prompt"
)

type countingSideQuery struct {
	calls int
	recs  []memory.Record
}

func (s *countingSideQuery) SelectForTurn(context.Context, identity.TenantScope, string, int) ([]memory.Record, error) {
	s.calls++
	return s.recs, nil
}

func TestPrepareTurnMemoryLegacyPreservesSideQuery(t *testing.T) {
	side := &countingSideQuery{recs: []memory.Record{
		{ID: "bond:user:mudnet", Content: "compact bond", Metadata: map[string]string{
			"kind": "bond", "bond_slots": "basics,boundaries", "bond_item_ids": "b1,b2",
		}},
		{ID: "scene:s1", Content: "scene norm", Metadata: map[string]string{
			"kind": "scene_norm", "scene_ids": "s1",
		}},
		{ID: "ep1", Content: "episode body", Metadata: map[string]string{
			"kind": "event", "about": "user:mudnet",
		}},
	}}
	svc := &Service{Scope: identity.LocalCLI(), RecallMode: RecallModeLegacy, SideQuery: side}

	got := svc.prepareTurnMemory(context.Background(), "query")
	if side.calls != 1 {
		t.Fatalf("side calls=%d, want 1", side.calls)
	}
	if got.bondNorm != "compact bond\n\nscene norm" || len(got.recallLines) != 1 {
		t.Fatalf("legacy memory changed: %+v", got)
	}
	if len(got.recallIDs) != 3 || len(got.sceneIDs) != 1 || len(got.bondItemIDs) != 2 {
		t.Fatalf("legacy trace changed: %+v", got)
	}
}

func TestPrepareTurnMemoryAgentSkipsSideQuery(t *testing.T) {
	scope := identity.LocalCLI()
	side := &countingSideQuery{recs: []memory.Record{{ID: "ep1", Content: "must not inject"}}}
	reader := &fakeNormReader{bond: graph.Bond{
		PersonID: scope.PersonID(), Version: 3,
		Items: []graph.BondItem{{ID: "p", Slot: graph.SlotPriorities, Claim: "完整常模可见", Status: "active"}},
	}}
	svc := &Service{
		Scope: scope, RecallMode: RecallModeAgent, SideQuery: side,
		Norms: NewNormSnapshotCache(reader, scope),
	}

	got := svc.prepareTurnMemory(context.Background(), "与关键词无关")
	if side.calls != 0 {
		t.Fatalf("agent mode called side query %d times", side.calls)
	}
	if len(got.recallLines) != 0 || prompt.FormatMemoryRecall(got.recallLines) != "" {
		t.Fatalf("agent mode auto-injected memory: %+v", got)
	}
	if got.normVersion != 3 || got.bondNorm == "" || got.recallGuidance == "" {
		t.Fatalf("agent context incomplete: %+v", got)
	}
}

func TestAgentRecallGuidanceStaysCompactWithoutLosingBoundaries(t *testing.T) {
	guidance := promptAgentContextSourceGuidance + promptAgentAttentionGuidance + promptAgentRecallGuidance
	if count := utf8.RuneCountInString(guidance); count > 1900 {
		t.Fatalf("agent recall guidance grew to %d runes", count)
	}
	for _, required := range []string{
		"Bond=关系 baseline", "SceneNorm=场景 guidance", "Workspace=当前 task",
		"Intent=未来 plan", "Proposal=未确认 hypothesis", "Episode=过去 evidence",
		"用户当前明确表达优先", "候选卡不是正文", "report_recall_evidence",
		"report_context_use", "report_context_focus", "槽满", "idle_turns",
		"即使此前无会话焦点", "switch 仅用于已有会话焦点", "主项目+辅助提醒",
		"同轮最多换词一次", "不回退最近 Episode", "隐藏思维",
	} {
		if !strings.Contains(guidance, required) {
			t.Fatalf("compressed guidance lost boundary %q", required)
		}
	}
}
