package observe_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/observe"
)

func TestAttentionTraceAndContextEligibilityAreTurnScoped(t *testing.T) {
	ctx, attentionCollector := observe.WithAttentionHooks(context.Background(), observe.AttentionHooks{})
	ctx, contextCollector := observe.WithContextHooks(ctx, observe.ContextHooks{})
	observe.RecordContextCandidate(ctx, observe.ContextCandidateTrace{
		Source: contextsource.Episode, Operation: "search", ResultIDs: []string{"ep_1"},
	})
	observe.RecordContextRead(ctx, observe.ContextReadTrace{Source: contextsource.Episode, ID: "ep_1"})
	if !observe.ContextCandidateKnown(ctx, contextsource.Episode, "ep_1") || !observe.ContextReadKnown(ctx, contextsource.Episode, "ep_1") {
		t.Fatal("current-turn public lifecycle was not available")
	}
	if observe.ContextCandidateKnown(context.Background(), contextsource.Episode, "ep_1") {
		t.Fatal("candidate lifecycle leaked outside the turn context")
	}

	snapshot := attention.Snapshot{Version: attention.Version, Revision: 2, Capacity: attention.DefaultCapacity(), Items: []attention.Item{{
		Source: contextsource.Episode, ID: "ep_1", Role: contextsource.Evidence, Tier: attention.Center, IdleTurns: 2,
	}}}
	observe.RecordAttentionSnapshot(ctx, snapshot)
	finalSnapshot := snapshot
	finalSnapshot.Items = []attention.Item{{Source: contextsource.Episode, ID: "ep_1", Role: contextsource.Evidence, Tier: attention.Center}}
	observe.RecordAttentionFinalSnapshot(ctx, finalSnapshot)
	observe.RecordAttentionDecision(ctx, observe.AttentionDecisionTrace{
		Source: contextsource.Episode, ID: "ep_1", From: attention.Support, To: attention.Center,
	})
	if attentionCollector.Snapshot() == nil || attentionCollector.FinalSnapshot() == nil || attentionCollector.Snapshot().Items[0].IdleTurns != 2 || attentionCollector.FinalSnapshot().Items[0].IdleTurns != 0 || len(attentionCollector.Decisions()) != 1 || len(contextCollector.Reads()) != 1 {
		t.Fatalf("unexpected collectors: snapshot=%+v final=%+v decisions=%+v reads=%+v", attentionCollector.Snapshot(), attentionCollector.FinalSnapshot(), attentionCollector.Decisions(), contextCollector.Reads())
	}
	raw, err := json.Marshal(observe.TurnTrace{Attention: attentionCollector.Snapshot(), AttentionFinal: attentionCollector.FinalSnapshot(), AttentionDecisions: attentionCollector.Decisions()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SECRET_BODY") || !strings.Contains(string(raw), `"tier":"center"`) || !strings.Contains(string(raw), `"attention_final"`) {
		t.Fatalf("invalid attention trace: %s", raw)
	}
}
