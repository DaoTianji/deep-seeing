package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
)

func TestManageAttentionUsesPublicLifecycleAndPersistsBySession(t *testing.T) {
	episodes, err := memory.NewEpisodeStore(filepath.Join(t.TempDir(), "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	workspace := attention.NewSessionStore(attention.Capacity{Center: 1, Support: 1, Periphery: 1})
	all, err := All(Deps{
		Scope: identity.LocalCLI(), Episodes: episodes, SessionID: "s1", RecallMode: "agent", Attention: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !toolNames(t, all)["manage_attention"] {
		t.Fatal("agent mode omitted manage_attention")
	}
	ctx, attentionCollector := observe.WithAttentionHooks(context.Background(), observe.AttentionHooks{})
	ctx, _ = observe.WithContextHooks(ctx, observe.ContextHooks{})
	observe.RecordContextCandidate(ctx, observe.ContextCandidateTrace{
		Source: contextsource.Episode, Operation: "search", ResultIDs: []string{"e1"},
	})

	unread, err := findInvokableTool(t, all, "manage_attention").InvokableRun(ctx,
		`{"decisions":[{"source":"episode","id":"e1","target":"center"}]}`)
	if err != nil || !strings.Contains(unread, `"ok":false`) {
		t.Fatalf("unread center accepted: output=%s err=%v", unread, err)
	}
	periphery, err := findInvokableTool(t, all, "manage_attention").InvokableRun(ctx,
		`{"decisions":[{"source":"episode","id":"e1","target":"periphery"}]}`)
	if err != nil || !strings.Contains(periphery, `"ok":true`) {
		t.Fatalf("candidate periphery rejected: output=%s err=%v", periphery, err)
	}
	observe.RecordContextRead(ctx, observe.ContextReadTrace{Source: contextsource.Episode, ID: "e1"})
	promoted, err := findInvokableTool(t, all, "manage_attention").InvokableRun(ctx,
		`{"decisions":[{"source":"episode","id":"e1","target":"center"}]}`)
	if err != nil || !strings.Contains(promoted, `"ok":true`) {
		t.Fatalf("read candidate promotion rejected: output=%s err=%v", promoted, err)
	}
	if snapshot := workspace.Snapshot("s1"); len(snapshot.Items) != 1 || snapshot.Items[0].Tier != attention.Center {
		t.Fatalf("attention not retained: %+v", snapshot)
	}
	if len(attentionCollector.Decisions()) != 2 {
		t.Fatalf("unexpected public decisions: %+v", attentionCollector.Decisions())
	}
}

func TestManageAttentionIsAgentOnly(t *testing.T) {
	episodes, _ := memory.NewEpisodeStore(filepath.Join(t.TempDir(), "episodes"))
	all, err := All(Deps{
		Scope: identity.LocalCLI(), Episodes: episodes, SessionID: "s", RecallMode: "legacy",
		Attention: attention.NewSessionStore(attention.DefaultCapacity()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if toolNames(t, all)["manage_attention"] {
		t.Fatal("legacy mode exposed manage_attention")
	}
}
