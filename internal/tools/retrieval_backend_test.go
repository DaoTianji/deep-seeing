package tools

import (
	"context"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"encoding/json"
	"strings"
	"testing"
)

func TestDerivedRetrievalPreservesReadAndEvidenceContract(t *testing.T) {
	scope := identity.LocalCLI()
	store, _ := memory.NewEpisodeStore(t.TempDir())
	ep, _ := store.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{Content: strings.Repeat("背景。", 100) + "杭州搬家 source", PersonIDs: []string{scope.PersonID()}})
	role, _ := store.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{Content: "杭州 backstage secret", RoleID: "role", ExperienceMode: memory.ExperienceSimulatedRoleplay})
	all, err := All(Deps{Scope: scope, Episodes: store, RecallMode: "agent", Retriever: &memory.EpisodeRetriever{Store: store, Scope: scope, Backend: "bm25"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, c := observe.WithRecallCollector(context.Background())
	out, err := findInvokableTool(t, all, "search_episodes").InvokableRun(ctx, `{"query":"杭州搬家"}`)
	if err != nil || !strings.Contains(out, ep.ID) || strings.Contains(out, ep.Content) {
		t.Fatalf("%s %v", out, err)
	}
	if len(c.Reads()) != 0 || len(c.Searches()) != 1 || c.Searches()[0].Backend != "bm25" {
		t.Fatal("bad trace")
	}
	out, err = findInvokableTool(t, all, "read_episode").InvokableRun(ctx, `{"id":"`+role.ID+`"}`)
	if err != nil || !strings.Contains(out, `"ok":false`) {
		t.Fatal("role access allowed")
	}
	if len(c.Reads()) != 1 || c.Reads()[0].Error == "" {
		t.Fatal("denied read recorded as success")
	}
	trace, _ := json.Marshal(c.Searches())
	if strings.Contains(string(trace), "source") {
		t.Fatal("trace leaked body")
	}
}
