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
	if !strings.Contains(recs[0].Metadata["bond_slots"], "basics") {
		t.Fatalf("meta=%v", recs[0].Metadata)
	}
	if recs[0].Metadata["strategy_omit"] != "1" {
		t.Fatalf("expected strategy_omit meta")
	}
	if recs[1].ID != "ep_1" {
		t.Fatalf("second=%v", recs[1])
	}
}

func TestBondAwareSideQueryEmptyPlaceholder(t *testing.T) {
	sq := &memory.BondAwareSideQuery{
		Graph: fakeBond{bond: graph.Bond{PersonID: "user:mudnet"}},
	}
	recs, err := sq.SelectForTurn(context.Background(), identity.LocalCLI(), "hi", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Metadata["placeholder"] != "1" {
		t.Fatalf("recs=%v", recs)
	}
	if !strings.Contains(recs[0].Content, "常模尚薄") {
		t.Fatalf("content=%s", recs[0].Content)
	}
}

func TestBondAwareSideQuerySceneBypass(t *testing.T) {
	store, err := memory.NewSceneStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	_, err = store.Write(scope, memory.SceneNorm{
		Title: "写代码", Keywords: []string{"golang"},
		Body: "- 在意错误处理",
	})
	if err != nil {
		t.Fatal(err)
	}
	sq := &memory.BondAwareSideQuery{
		Graph:  fakeBond{bond: graph.Bond{PersonID: scope.PersonID(), Basics: "早期同伴"}},
		Scenes: store,
	}
	recs, err := sq.SelectForTurn(context.Background(), scope, "这段 golang 怎么编译", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) < 2 {
		t.Fatalf("len=%d recs=%v", len(recs), recs)
	}
	if recs[0].Metadata["kind"] != "bond" {
		t.Fatalf("first=%v", recs[0])
	}
	if recs[1].Metadata["kind"] != "scene_norm" {
		t.Fatalf("second=%v", recs[1])
	}
	if !strings.Contains(recs[1].Content, "错误处理") {
		t.Fatalf("scene=%s", recs[1].Content)
	}
}
