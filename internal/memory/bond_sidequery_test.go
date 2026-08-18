package memory_test

import (
	"context"
	"strings"
	"testing"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

type fakeBond struct {
	bond graph.Bond
	err  error
}

func (f fakeBond) GetBond(context.Context, identity.TenantScope, string) (graph.Bond, error) {
	return f.bond, f.err
}

type fakeEpisodes struct {
	recs []memory.Record
}

func (f fakeEpisodes) SelectForTurn(context.Context, identity.TenantScope, string, int) ([]memory.Record, error) {
	return f.recs, nil
}

func TestBondAwareSideQueryOrdersBondFirst(t *testing.T) {
	sq := &memory.BondAwareSideQuery{
		Graph: fakeBond{bond: graph.Bond{
			PersonID: "user:mudnet", RoleAtOrigin: "friend / early companion", Basics: "早期同伴",
		}},
		Episodes: fakeEpisodes{recs: []memory.Record{{ID: "ep_1", Content: "某次对话"}}},
	}
	recs, err := sq.SelectForTurn(context.Background(), identity.LocalCLI(), "你好", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) < 2 {
		t.Fatalf("len=%d", len(recs))
	}
	if recs[0].Metadata["kind"] != "bond" {
		t.Fatalf("first=%v", recs[0])
	}
	if !strings.Contains(recs[0].Content, "早期同伴") {
		t.Fatalf("content=%s", recs[0].Content)
	}
	if recs[1].ID != "ep_1" {
		t.Fatalf("second=%v", recs[1])
	}
}
