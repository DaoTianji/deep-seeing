package theater

import (
	"context"
	"strings"
	"testing"

	"deep-seeing/internal/memory"
)

type fakeDirectorCompleter struct {
	out string
	err error
}

func (f fakeDirectorCompleter) Complete(context.Context, string, string) (string, error) {
	return f.out, f.err
}

func newDirectorFixture(t *testing.T) (*Store, *memory.EpisodeStore, RoleDefinition, RoleInstance, RoleSession) {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	episodes, err := memory.NewEpisodeStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, inst, session := readyRole(t, store)
	return store, episodes, d, inst, session
}

func TestDirectorObserveRecordsWithoutMutation(t *testing.T) {
	ctx := context.Background()
	store, episodes, d, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{
		Mode: ModeObserve, Store: store, Episodes: episodes, Scope: testScope(),
		Chat: fakeDirectorCompleter{out: "{\"action\":\"set_scene\",\"reason_code\":\"scene\",\"scene\":\"风暴中的甲板\",\"touches_canonical\":false}"},
	}
	action, err := reviewer.ReviewAndApply(ctx, DirectorReviewInput{Definition: d, Instance: inst, Session: session, UserText: "起风了", ActorAnswer: "抓紧绳索", TurnID: "turn_1"})
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != ActionExpected || action.Type != ActionSetScene {
		t.Fatalf("unexpected action: %#v", action)
	}
	current, _ := store.GetInstance(ctx, inst.ID)
	if current.Scene != "" || current.Version != inst.Version {
		t.Fatalf("observe mode mutated instance: %#v", current)
	}
	actions, _ := store.ListActions(ctx, session.ID)
	if len(actions) != 1 || actions[0].After["scene"] != "风暴中的甲板" {
		t.Fatalf("expected action missing: %#v", actions)
	}
}

func TestDirectorCanonicalChangeForksWorldline(t *testing.T) {
	ctx := context.Background()
	store, episodes, d, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{
		Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope(),
		Chat: fakeDirectorCompleter{out: "{\"action\":\"revise_role_model\",\"reason_code\":\"canonical_change\",\"state_key\":\"occupation\",\"state_value\":\"医生\",\"label\":\"另一种人生\",\"touches_canonical\":true}"},
	}
	action, err := reviewer.ReviewAndApply(ctx, DirectorReviewInput{Definition: d, Instance: inst, Session: session, TurnID: "turn_2"})
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != ActionApplied || action.WorldlineID == session.WorldlineID {
		t.Fatalf("canonical action did not fork: %#v", action)
	}
	parent, err := store.GetWorldline(ctx, session.WorldlineID)
	if err != nil {
		t.Fatal(err)
	}
	if parent.State["occupation"] != "" {
		t.Fatalf("parent worldline overwritten: %#v", parent)
	}
	branch, err := store.GetWorldline(ctx, action.WorldlineID)
	if err != nil {
		t.Fatal(err)
	}
	if branch.ParentWorldlineID != parent.ID || branch.State["occupation"] != "医生" {
		t.Fatalf("branch missing revision: %#v", branch)
	}
}

func TestDirectorActionRevertIsCompensating(t *testing.T) {
	ctx := context.Background()
	store, episodes, d, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{
		Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope(),
		Chat: fakeDirectorCompleter{out: "{\"action\":\"set_scene\",\"reason_code\":\"scene\",\"scene\":\"图书馆\",\"touches_canonical\":false}"},
	}
	action, err := reviewer.ReviewAndApply(ctx, DirectorReviewInput{Definition: d, Instance: inst, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	reverted, err := reviewer.Revert(ctx, session.ID, action.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reverted.RevertsActionID != action.ID || reverted.Status != ActionReverted {
		t.Fatalf("bad compensating action: %#v", reverted)
	}
	current, _ := store.GetInstance(ctx, inst.ID)
	if current.Scene != "" {
		t.Fatalf("scene not restored: %#v", current)
	}
	actions, _ := store.ListActions(ctx, session.ID)
	if len(actions) != 2 {
		t.Fatalf("history was not append-only: %#v", actions)
	}
}

func TestInvalidDirectorOutputFallsBackNoChange(t *testing.T) {
	ctx := context.Background()
	store, episodes, d, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{
		Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope(),
		Chat: fakeDirectorCompleter{out: "I refuse JSON"},
	}
	action, err := reviewer.ReviewAndApply(ctx, DirectorReviewInput{Definition: d, Instance: inst, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != ActionNoChange || action.Status != ActionApplied {
		t.Fatalf("unsafe fallback: %#v", action)
	}
}

func TestCanonicalRevertReturnsToParentWithoutDeletingBranch(t *testing.T) {
	ctx := context.Background()
	store, episodes, d, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{
		Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope(),
		Chat: fakeDirectorCompleter{out: "{\"action\":\"revise_role_model\",\"reason_code\":\"canonical_change\",\"state_key\":\"occupation\",\"state_value\":\"医生\",\"touches_canonical\":true}"},
	}
	action, err := reviewer.ReviewAndApply(ctx, DirectorReviewInput{Definition: d, Instance: inst, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	branchID := action.WorldlineID
	if _, err := reviewer.Revert(ctx, session.ID, action.ID); err != nil {
		t.Fatal(err)
	}
	current, _ := store.GetInstance(ctx, inst.ID)
	if current.CurrentWorldlineID != session.WorldlineID {
		t.Fatalf("did not return to parent: %#v", current)
	}
	if _, err := store.GetWorldline(ctx, branchID); err != nil {
		t.Fatal("revert deleted branch history", err)
	}
}

func TestDirectorRevertRejectsLaterWorldlineChange(t *testing.T) {
	ctx := context.Background()
	store, episodes, d, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{
		Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope(),
		Chat: fakeDirectorCompleter{out: "{\"action\":\"fork_worldline\",\"reason_code\":\"user_request\",\"label\":\"分支\",\"touches_canonical\":true}"},
	}
	action, err := reviewer.ReviewAndApply(ctx, DirectorReviewInput{Definition: d, Instance: inst, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	world, _ := store.GetWorldline(ctx, action.After["worldline_id"])
	if _, _, err := store.SetWorldlineState(ctx, session.ID, world.Version, "later", "change"); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewer.Revert(ctx, session.ID, action.ID); err == nil {
		t.Fatal("revert overwrote later worldline change")
	}
}

func TestDirectorPromptKeepsOneTurnInstructionEphemeral(t *testing.T) {
	if !strings.Contains(directorReviewSystem, "一次性任务要求不得写入持续 role_state") {
		t.Fatal("director prompt no longer protects ephemeral instructions")
	}
	if !strings.Contains(directorReviewSystem, "不得把 source-backed/canonical 事实") {
		t.Fatal("director prompt no longer prevents canonical duplication")
	}
	if !strings.Contains(directorReviewSystem, "氛围描写、天气、普通闲聊") {
		t.Fatal("director prompt no longer protects stable scene small talk")
	}
}

func TestBackstageRequestedInterventionUsesObserveLedger(t *testing.T) {
	ctx := context.Background()
	store, episodes, _, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{Mode: ModeObserve, Store: store, Episodes: episodes, Scope: testScope()}
	action, err := reviewer.ApplyRequested(ctx, session.ID, DirectorActionRequest{
		Action: ActionSetScene, ReasonCode: "user_request", Scene: "旧灯塔",
	})
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != ActionExpected || action.After["scene"] != "旧灯塔" {
		t.Fatalf("unexpected observed action: %#v", action)
	}
	current, _ := store.GetInstance(ctx, inst.ID)
	if current.Scene != "" || current.Version != inst.Version {
		t.Fatalf("observe mutated role: %#v", current)
	}
	actions, _ := store.ListActions(ctx, session.ID)
	if len(actions) != 1 || actions[0].ID != action.ID {
		t.Fatalf("backstage action missing from ledger: %#v", actions)
	}
}

func TestAgentRequestedInterventionChangesNextActorPrompt(t *testing.T) {
	ctx := context.Background()
	store, episodes, definition, _, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope()}
	action, err := reviewer.ApplyRequested(ctx, session.ID, DirectorActionRequest{
		Action: " SET_ROLE_STATE ", ReasonCode: " USER_REQUEST ",
		StateKey:   "performance_style",
		StateValue: "高能、尖锐、戏剧化；主动反问，与原本平淡状态形成明显反差。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != ActionApplied || action.AppliedVersion <= action.ExpectedVersion || !action.ReadbackVerified {
		t.Fatalf("intervention was not applied: %#v", action)
	}
	_, instance, activeSession, err := store.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	world, err := store.GetWorldline(ctx, activeSession.WorldlineID)
	if err != nil {
		t.Fatal(err)
	}
	prompt := BuildActorPrompt(definition, instance, world, nil)
	for _, expected := range []string{"performance_style", "高能、尖锐、戏剧化", "主动反问"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("next actor prompt missing applied intervention %q: %s", expected, prompt)
		}
	}
}

func TestPerformanceDirectiveRevertRestoresPriorState(t *testing.T) {
	ctx := context.Background()
	store, episodes, _, _, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope()}
	action, err := reviewer.ApplyRequested(ctx, session.ID, DirectorActionRequest{Action: ActionSetRoleState, ReasonCode: "user_request", StateKey: "performance_style", StateValue: "强势追问", Scope: "session", Intensity: 9})
	if err != nil || action.Status != ActionApplied {
		t.Fatalf("apply: %#v %v", action, err)
	}
	reverted, err := reviewer.Revert(ctx, session.ID, action.ID)
	if err != nil || reverted.Status != ActionReverted {
		t.Fatalf("revert: %#v %v", reverted, err)
	}
	current, _ := store.GetInstance(ctx, action.RoleInstanceID)
	if current.Performance != nil {
		t.Fatalf("performance directive survived revert: %#v", current.Performance)
	}
}

func TestRejectedInterventionIsActionableAndRecorded(t *testing.T) {
	ctx := context.Background()
	store, episodes, _, _, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope()}
	action, err := reviewer.ApplyRequested(ctx, session.ID, DirectorActionRequest{
		Action: ActionSetRoleState, ReasonCode: "style",
	})
	if err == nil || !strings.Contains(err.Error(), "reason_code") || !strings.Contains(err.Error(), "user_request") {
		t.Fatalf("expected actionable error, got action=%#v err=%v", action, err)
	}
	if action.Status != ActionRejected || action.After["error"] == "" {
		t.Fatalf("rejection was not recorded: %#v", action)
	}
	actions, _ := store.ListActions(ctx, session.ID)
	if len(actions) != 1 || actions[0].Status != ActionRejected {
		t.Fatalf("rejected ledger mismatch: %#v", actions)
	}
}

func TestNoChangeCannotForkByClaimingCanonicalImpact(t *testing.T) {
	ctx := context.Background()
	store, episodes, d, inst, session := newDirectorFixture(t)
	reviewer := &DirectorReviewer{
		Mode: ModeAgent, Store: store, Episodes: episodes, Scope: testScope(),
		Chat: fakeDirectorCompleter{out: `{"action":"no_change","reason_code":"no_material_reason","touches_canonical":true}`},
	}
	action, err := reviewer.ReviewAndApply(ctx, DirectorReviewInput{Definition: d, Instance: inst, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	if action.TouchesCanonical || action.WorldlineID != session.WorldlineID {
		t.Fatalf("no_change gained canonical authority: %#v", action)
	}
	worlds, _ := store.ListWorldlines(ctx, inst.ID)
	if len(worlds) != 1 {
		t.Fatalf("no_change created worldline: %#v", worlds)
	}
}
