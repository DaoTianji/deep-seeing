package memory_test

import (
	"strings"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

func TestSceneStoreWriteMatch(t *testing.T) {
	store, err := memory.NewSceneStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	sc, err := store.Write(scope, memory.SceneNorm{
		Title: "写代码", Keywords: []string{"golang", "编译", "代码"},
		Body: "- 在意错误处理\n- 讨厌无类型的草稿",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sc.ID == "" {
		t.Fatal("id")
	}
	hit, err := store.MatchQuery(scope.PersonID(), "帮我看这段 golang 代码", 5)
	if err != nil || len(hit) != 1 {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	miss, err := store.MatchQuery(scope.PersonID(), "今天晚饭吃什么", 5)
	if err != nil || len(miss) != 0 {
		t.Fatalf("miss=%v err=%v", miss, err)
	}
	text := memory.FormatSceneRecall(hit)
	if !strings.Contains(text, "错误处理") {
		t.Fatalf("%s", text)
	}
}
