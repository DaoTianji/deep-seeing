package selfmodel

import (
	"context"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

func TestReflectionContextOnlyReturnsActivePatternAndTensionCards(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pattern, _ := store.Create(Write{Type: TypePattern, Title: "反复确认", Body: "完整正文不应被薄快照直接返回", Status: StatusTentative, ExperienceModes: []memory.ExperienceMode{memory.ExperienceRealInteraction}})
	tension, _ := store.Create(Write{Type: TypeTension, Title: "谨慎与主动", Body: "两股力量仍未解决", Status: StatusTentative, ExperienceModes: []memory.ExperienceMode{memory.ExperienceRealInteraction}})
	_, _ = store.Create(Write{Type: TypePrinciple, Title: "原则候选", Body: "本轮不进入反思上下文", Status: StatusTentative, ExperienceModes: []memory.ExperienceMode{memory.ExperienceRealInteraction}})
	deprecated, _ := store.Create(Write{Type: TypePattern, Title: "旧模式", Body: "已撤销", Status: StatusTentative, ExperienceModes: []memory.ExperienceMode{memory.ExperienceRealInteraction}})
	_, _, _ = store.Deprecate(deprecated.ID, "过时", "test")

	items, err := (DreamBridge{Store: store}).ReflectionContext(context.Background(), identity.LocalCLI(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items=%+v", items)
	}
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[pattern.ID] || !ids[tension.ID] || ids[deprecated.ID] {
		t.Fatalf("items=%+v", items)
	}
}
