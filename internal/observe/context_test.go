package observe_test

import (
	"context"
	"testing"

	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/observe"
)

func TestContextLifecycleRequiresCandidateAndRead(t *testing.T) {
	ctx, collector := observe.WithContextHooks(context.Background(), observe.ContextHooks{})
	if err := observe.RecordContextUse(ctx, observe.ContextUseTrace{
		Source: contextsource.SceneNorm, ID: "scene_1", Disposition: "used",
	}); err == nil {
		t.Fatal("accepted use without candidate")
	}
	observe.RecordContextCandidate(ctx, observe.ContextCandidateTrace{
		Source: contextsource.SceneNorm, Operation: "list", ResultIDs: []string{"scene_1", "scene_1"},
	})
	if err := observe.RecordContextUse(ctx, observe.ContextUseTrace{
		Source: contextsource.SceneNorm, ID: "scene_1", Disposition: "used",
	}); err == nil {
		t.Fatal("accepted use without read")
	}
	observe.RecordContextRead(ctx, observe.ContextReadTrace{Source: contextsource.SceneNorm, ID: "scene_1"})
	if err := observe.RecordContextUse(ctx, observe.ContextUseTrace{
		Source: contextsource.SceneNorm, ID: "scene_1", Disposition: "used",
	}); err != nil {
		t.Fatalf("read candidate was rejected: %v", err)
	}
	if err := observe.RecordContextUse(ctx, observe.ContextUseTrace{
		Source: contextsource.SceneNorm, ID: "scene_1", Disposition: "dismissed", ReasonCode: "irrelevant",
	}); err == nil {
		t.Fatal("accepted duplicate decision")
	}
	if got := collector.Candidates(); len(got) != 1 || len(got[0].ResultIDs) != 1 {
		t.Fatalf("candidate ids were not bounded and deduplicated: %+v", got)
	}
	if got := collector.Uses(); len(got) != 1 || got[0].Role != contextsource.Guidance {
		t.Fatalf("unexpected uses: %+v", got)
	}
}

func TestContextDismissalAndTurnIsolation(t *testing.T) {
	ctx1, first := observe.WithContextHooks(context.Background(), observe.ContextHooks{})
	observe.RecordContextCandidate(ctx1, observe.ContextCandidateTrace{
		Source: contextsource.Proposal, Operation: "list", ResultIDs: []string{"prop_1"},
	})
	if err := observe.RecordContextUse(ctx1, observe.ContextUseTrace{
		Source: contextsource.Proposal, ID: "prop_1", Disposition: "dismissed", ReasonCode: "unconfirmed",
	}); err != nil {
		t.Fatal(err)
	}
	if err := observe.RecordContextUse(ctx1, observe.ContextUseTrace{
		Source: contextsource.Proposal, ID: "prop_2", Role: contextsource.Evidence,
		Disposition: "dismissed", ReasonCode: "made_up",
	}); err == nil {
		t.Fatal("accepted invalid role/reason")
	}

	ctx2, second := observe.WithContextHooks(context.Background(), observe.ContextHooks{})
	observe.RecordContextRead(ctx2, observe.ContextReadTrace{Source: contextsource.Proposal, ID: "prop_1"})
	if len(first.Reads()) != 0 || len(second.Reads()) != 1 {
		t.Fatalf("turn collectors leaked: first=%+v second=%+v", first.Reads(), second.Reads())
	}
	if len(first.Uses()) != 1 || len(second.Uses()) != 0 {
		t.Fatalf("turn use state leaked: first=%+v second=%+v", first.Uses(), second.Uses())
	}
}
