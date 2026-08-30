package theater

import (
	"context"
	"strings"
	"testing"

	"deep-seeing/internal/memory"
	"deep-seeing/internal/runtime"
)

type fakeTurnService struct {
	answer string
	inputs []string
	onCall func()
}

func (f *fakeTurnService) StreamTurnWithHooks(_ context.Context, input string, hooks runtime.TurnHooks) (runtime.TurnResult, error) {
	f.inputs = append(f.inputs, input)
	if f.onCall != nil {
		f.onCall()
	}
	if hooks.WriteDelta != nil {
		hooks.WriteDelta(f.answer)
	}
	return runtime.TurnResult{TurnID: "turn_test", Answer: f.answer}, nil
}

func readyRole(t *testing.T, store *Store) (RoleDefinition, RoleInstance, RoleSession) {
	t.Helper()
	ctx := context.Background()
	d, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "林舟", Identity: "虚构航海者"})
	if err != nil {
		t.Fatal(err)
	}
	d, _ = store.SetValidation(ctx, d.ID, ValidationReport{Passed: true})
	d, _ = store.Publish(ctx, d.ID)
	_, inst, session, err := store.Enter(ctx, testScope(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d, inst, session
}

func TestRouterSeparatesStageAndBackstage(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(t.TempDir())
	_, _, session := readyRole(t, store)
	normal := &fakeTurnService{answer: "normal"}
	actor := &fakeTurnService{answer: "角色回答"}
	director := &fakeTurnService{answer: "安的幕后回答"}
	router := &Router{
		Mode: ModeAgent, Store: store, Normal: normal, Director: director,
		Actors: ActorBuilderFunc(func(context.Context, RoleDefinition, RoleInstance, RoleSession) (TurnService, error) {
			return actor, nil
		}),
	}
	var stageOut strings.Builder
	if _, err := router.StreamTurnWithHooks(ctx, ChannelStage, session.ID, "你记得海吗？", RouterHooks{
		Turn: runtime.TurnHooks{WriteDelta: func(v string) { stageOut.WriteString(v) }},
	}); err != nil {
		t.Fatal(err)
	}
	if stageOut.String() != "角色回答" || len(actor.inputs) != 1 || actor.inputs[0] != "你记得海吗？" {
		t.Fatalf("bad stage route: out=%q inputs=%#v", stageOut.String(), actor.inputs)
	}
	var backOut strings.Builder
	if _, err := router.StreamTurnWithHooks(ctx, ChannelBackstage, session.ID, "安，观察他的迟疑。", RouterHooks{
		Turn: runtime.TurnHooks{WriteDelta: func(v string) { backOut.WriteString(v) }},
	}); err != nil {
		t.Fatal(err)
	}
	if backOut.String() != "安的幕后回答" || len(director.inputs) != 1 || !strings.Contains(director.inputs[0], "安，观察他的迟疑") {
		t.Fatalf("bad backstage route: out=%q inputs=%#v", backOut.String(), director.inputs)
	}
	if strings.Contains(actor.inputs[0], "幕后") || strings.Contains(actor.inputs[0], "安，观察") {
		t.Fatalf("backstage leaked to actor: %q", actor.inputs[0])
	}
	stage, _ := store.ReadTranscript(ctx, session.ID, ChannelStage, 10)
	backstage, _ := store.ReadTranscript(ctx, session.ID, ChannelBackstage, 10)
	if len(stage) != 2 || len(backstage) != 2 {
		t.Fatalf("transcripts missing: stage=%d backstage=%d", len(stage), len(backstage))
	}
}

func TestRouterNormalAndModeOffCompatibility(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(t.TempDir())
	normal := &fakeTurnService{answer: "安"}
	router := &Router{Mode: ModeAgent, Store: store, Normal: normal}
	if _, err := router.StreamTurnWithHooks(ctx, ChannelStage, "", "你好", RouterHooks{}); err != nil {
		t.Fatal(err)
	}
	readyRole(t, store)
	router.Mode = ModeOff
	if _, err := router.StreamTurnWithHooks(ctx, ChannelStage, "", "仍然找安", RouterHooks{}); err != nil {
		t.Fatal(err)
	}
	if len(normal.inputs) != 2 || normal.inputs[1] != "仍然找安" {
		t.Fatalf("normal compatibility broken: %#v", normal.inputs)
	}
}

func TestPausedRoleRejectsStageButAllowsBackstage(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(t.TempDir())
	_, _, session := readyRole(t, store)
	if _, err := store.Pause(ctx, "user"); err != nil {
		t.Fatal(err)
	}
	actor := &fakeTurnService{answer: "不该发生"}
	director := &fakeTurnService{answer: "已暂停"}
	router := &Router{
		Mode: ModeAgent, Store: store, Director: director,
		Actors: ActorBuilderFunc(func(context.Context, RoleDefinition, RoleInstance, RoleSession) (TurnService, error) {
			return actor, nil
		}),
	}
	if _, err := router.StreamTurnWithHooks(ctx, ChannelStage, session.ID, "继续", RouterHooks{}); err == nil {
		t.Fatal("paused role accepted stage turn")
	}
	if _, err := router.StreamTurnWithHooks(ctx, ChannelBackstage, session.ID, "安，我们先停一下", RouterHooks{}); err != nil {
		t.Fatal(err)
	}
	if len(actor.inputs) != 0 || len(director.inputs) != 1 {
		t.Fatalf("paused routing wrong actor=%d director=%d", len(actor.inputs), len(director.inputs))
	}
}

func TestActorControlLeakIsNotPersisted(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(t.TempDir())
	_, _, session := readyRole(t, store)
	actor := &fakeTurnService{answer: "我看到了 RoleInstance 的内部信息"}
	router := &Router{
		Mode: ModeAgent, Store: store,
		Actors: ActorBuilderFunc(func(context.Context, RoleDefinition, RoleInstance, RoleSession) (TurnService, error) {
			return actor, nil
		}),
	}
	var out strings.Builder
	if _, err := router.StreamTurnWithHooks(ctx, ChannelStage, session.ID, "你是谁", RouterHooks{Turn: runtime.TurnHooks{WriteDelta: func(v string) { out.WriteString(v) }}}); err == nil {
		t.Fatal("expected control leak rejection")
	}
	if out.Len() != 0 {
		t.Fatalf("leaked output was streamed: %q", out.String())
	}
	stage, _ := store.ReadTranscript(ctx, session.ID, ChannelStage, 10)
	if len(stage) != 0 {
		t.Fatalf("leaked answer persisted: %#v", stage)
	}
}

func TestRouterEmitsRoleMemoryOnlyAfterSuccessfulWrite(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(t.TempDir())
	episodes, _ := memory.NewEpisodeStore(t.TempDir())
	d, inst, session := readyRole(t, store)
	actor := &fakeTurnService{answer: "我会记住。"}
	actor.onCall = func() {
		_, _ = episodes.WriteEpisode(ctx, testScope(), memory.EpisodeWrite{
			Content: "台前形成的新经历", ExperienceMode: memory.ExperienceSimulatedRoleplay,
			RoleID: d.ID, RoleInstanceID: inst.ID, RoleSessionID: session.ID,
			WorldlineID: session.WorldlineID, RoleMemoryClass: string(MemorySimulated),
		})
	}
	router := &Router{
		Mode: ModeAgent, Store: store, Scope: testScope(),
		Reviewer: &DirectorReviewer{Mode: ModeObserve, Store: store, Episodes: episodes, Scope: testScope()},
		Actors: ActorBuilderFunc(func(context.Context, RoleDefinition, RoleInstance, RoleSession) (TurnService, error) {
			return actor, nil
		}),
	}
	var events []RoleEvent
	_, err := router.StreamTurnWithHooks(ctx, ChannelStage, session.ID, "请记住", RouterHooks{OnRoleEvent: func(event RoleEvent) { events = append(events, event) }})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type == "role_memory_written" {
			found = true
			if event.Data["episode_id"] == "" {
				t.Fatalf("memory event missing id: %#v", event)
			}
		}
	}
	if !found {
		t.Fatalf("memory event missing: %#v", events)
	}
}
