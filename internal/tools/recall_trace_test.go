package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
)

func TestSearchEpisodesRecordsIDsWithoutContent(t *testing.T) {
	scope := identity.LocalCLI()
	store, err := memory.NewEpisodeStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ep, err := store.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{
		Content: "项目暗号 SECRET_EPISODE_BODY", PersonIDs: []string{scope.PersonID()},
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := All(Deps{Scope: scope, Episodes: store, RecallMode: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	search := findInvokableTool(t, all, "search_episodes")
	ctx, collector := observe.WithRecallCollector(context.Background())
	searchOut, err := search.InvokableRun(ctx, `{"query":"项目暗号","limit":3}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(searchOut, "SECRET_EPISODE_BODY") || strings.Contains(searchOut, `"content"`) {
		t.Fatalf("search leaked episode body: %s", searchOut)
	}
	if !strings.Contains(searchOut, `"candidates"`) || strings.Contains(searchOut, `"episodes"`) {
		t.Fatalf("search did not return candidate cards: %s", searchOut)
	}
	if _, err := search.InvokableRun(ctx, `{"query":"🦄🪐","limit":3}`); err != nil {
		t.Fatal(err)
	}

	events := collector.Searches()
	if len(events) != 2 || events[0].ResultCount != 1 || len(events[0].ResultIDs) != 1 || events[0].ResultIDs[0] != ep.ID {
		t.Fatalf("events=%+v", events)
	}
	if events[1].ResultCount != 0 || len(events[1].ResultIDs) != 0 {
		t.Fatalf("empty search fell back to an episode: %+v", events[1])
	}
	raw, err := json.Marshal(observe.TurnTrace{RecallSearches: events})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SECRET_EPISODE_BODY") {
		t.Fatalf("episode body leaked into trace: %s", raw)
	}
}

func TestReadEpisodeAndEvidenceDeclarationAreRecorded(t *testing.T) {
	scope := identity.LocalCLI()
	store, err := memory.NewEpisodeStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ep, err := store.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{Content: "T2 证据正文"})
	if err != nil {
		t.Fatal(err)
	}
	all, err := All(Deps{Scope: scope, Episodes: store, RecallMode: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, collector := observe.WithRecallCollector(context.Background())
	search := findInvokableTool(t, all, "search_episodes")
	if _, err := search.InvokableRun(ctx, `{"query":"T2","limit":3}`); err != nil {
		t.Fatal(err)
	}
	read := findInvokableTool(t, all, "read_episode")
	if _, err := read.InvokableRun(ctx, `{"id":"`+ep.ID+`"}`); err != nil {
		t.Fatal(err)
	}
	report := findInvokableTool(t, all, "report_recall_evidence")
	out, err := report.InvokableRun(ctx, `{"decisions":[{"episode_id":"`+ep.ID+`","status":"used"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"ok":true`) || len(collector.Reads()) != 1 || len(collector.Evidence()) != 1 {
		t.Fatalf("out=%s reads=%+v evidence=%+v", out, collector.Reads(), collector.Evidence())
	}
	raw, _ := json.Marshal(observe.TurnTrace{RecallReads: collector.Reads(), RecallEvidence: collector.Evidence()})
	if strings.Contains(string(raw), "T2 证据正文") {
		t.Fatalf("trace leaked body: %s", raw)
	}
}

func TestInspectRuntimeReportsRecallMode(t *testing.T) {
	store, err := memory.NewEpisodeStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	all, err := All(Deps{Scope: identity.LocalCLI(), Episodes: store, RecallMode: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := findInvokableTool(t, all, "inspect_runtime").InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"recall_mode":"agent"`) {
		t.Fatalf("recall mode missing from runtime snapshot: %s", raw)
	}
}

func TestBondMutationToolsNotifySnapshotInvalidation(t *testing.T) {
	scope := identity.LocalCLI()
	store, err := memory.NewEpisodeStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	all, err := All(Deps{
		Scope: scope, Episodes: store, Graph: &fakeGraphTools{},
		OnBondChanged: func() { changed++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args string
	}{
		{name: "set_explicit_bond_fact", args: `{"kind":"basics_fact","value":"喜欢直接沟通"}`},
		{name: "append_bond_boundary", args: `{"claim":"不要记录隐私"}`},
		{name: "set_bond_strategy_cache", args: `{"text":"先确认目标"}`},
	}
	for _, tc := range cases {
		if _, err := findInvokableTool(t, all, tc.name).InvokableRun(context.Background(), tc.args); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if changed != len(cases) {
		t.Fatalf("change notifications=%d, want %d", changed, len(cases))
	}
}

func findInvokableTool(t *testing.T, all []einotool.BaseTool, name string) einotool.InvokableTool {
	t.Helper()
	for _, candidate := range all {
		info, err := candidate.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.Name != name {
			continue
		}
		invokable, ok := candidate.(einotool.InvokableTool)
		if !ok {
			t.Fatalf("tool %s is not invokable", name)
		}
		return invokable
	}
	t.Fatalf("tool %s not found", name)
	return nil
}

type fakeGraphTools struct{}

func (*fakeGraphTools) GetBond(context.Context, identity.TenantScope, string) (graph.Bond, error) {
	return graph.Bond{PersonID: identity.LocalCLI().PersonID(), Version: 1}, nil
}

func (*fakeGraphTools) PatchBond(context.Context, identity.TenantScope, string, graph.BondPatch) (graph.Bond, error) {
	return graph.Bond{Version: 2}, nil
}

func (*fakeGraphTools) AppendBondItem(_ context.Context, _ identity.TenantScope, _ string, spec graph.AppendItemSpec) (graph.Bond, graph.BondItem, error) {
	return graph.Bond{Version: 2}, graph.BondItem{ID: "item", Slot: spec.Slot, Claim: spec.Claim, Status: "active"}, nil
}

func (*fakeGraphTools) SetBondStrategyCache(context.Context, identity.TenantScope, string, string) (graph.Bond, error) {
	return graph.Bond{Version: 2, StrategyCacheVer: 2}, nil
}

func (*fakeGraphTools) UpsertEpisodePointer(context.Context, identity.TenantScope, graph.EpisodePointer) error {
	return nil
}

func (*fakeGraphTools) UpsertCalls(context.Context, identity.TenantScope, string, string, string) error {
	return nil
}

func (*fakeGraphTools) MarkEpisodeStatus(context.Context, string, string, string) error {
	return nil
}
