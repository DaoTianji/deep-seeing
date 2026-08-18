package memory_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/transcript"
)

func TestSTMBounds(t *testing.T) {
	stm := memory.NewSTM(4)
	if err := stm.Append("s1", transcript.User("1"), transcript.Assistant("a1")); err != nil {
		t.Fatal(err)
	}
	if err := stm.Append("s1", transcript.User("2"), transcript.Assistant("a2")); err != nil {
		t.Fatal(err)
	}
	if err := stm.Append("s1", transcript.User("3"), transcript.Assistant("a3")); err != nil {
		t.Fatal(err)
	}
	got, err := stm.Get("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("len=%d want 4", len(got))
	}
	if got[0].Content != "2" || got[3].Content != "a3" {
		t.Fatalf("unexpected bound slice: %+v", got)
	}
}

func TestEpisodeStoreWriteAndSearch(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewEpisodeStore(filepath.Join(dir, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	ctx := context.Background()
	ep, err := store.WriteEpisode(ctx, scope, memory.EpisodeWrite{
		Kind:      memory.EpisodePreference,
		Content:   "mudnet 说希望被叫 mudnet；约定：叫助手安。",
		Why:       "称呼对我很重要",
		PersonIDs: []string{scope.PersonID()},
		SessionID: "cli",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ep.ID, "ep_") {
		t.Fatalf("id=%s", ep.ID)
	}
	index, err := store.IndexText(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(index, ep.ID) {
		t.Fatalf("index missing id: %s", index)
	}
	got, err := store.Get(ctx, ep.ID)
	if err != nil || !strings.Contains(got.Content, "安") {
		t.Fatalf("get: %v %+v", err, got)
	}
	found, err := store.Search(ctx, scope, memory.Query{Text: "安", Limit: 5})
	if err != nil || len(found) == 0 {
		t.Fatalf("search: %v %+v", err, found)
	}
}

func TestEpisodeMigratesTopics(t *testing.T) {
	root := t.TempDir()
	topics := filepath.Join(root, "topics")
	if err := os.MkdirAll(topics, 0o755); err != nil {
		t.Fatal(err)
	}
	topic := "---\nkey: assistant_name_an\ncategory: user\n---\n\n用户将助手命名为安。\n"
	if err := os.WriteFile(filepath.Join(topics, "assistant_name_an.md"), []byte(topic), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	eps, err := store.ListRecentEpisodes(context.Background(), scope, 5)
	if err != nil || len(eps) == 0 {
		t.Fatalf("migrate: %v %+v", err, eps)
	}
	if eps[0].LegacyKey != "assistant_name_an" {
		t.Fatalf("legacy_key=%q", eps[0].LegacyKey)
	}
	if _, err := os.Stat(filepath.Join(root, "episodes", ".migrated_from_topics")); err != nil {
		t.Fatal(err)
	}
}

func TestLLMSideQueryFallbackWithoutChat(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewEpisodeStore(filepath.Join(dir, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	ctx := context.Background()
	_, _ = store.WriteEpisode(ctx, scope, memory.EpisodeWrite{Kind: memory.EpisodeEvent, Content: "事实一"})
	side := &memory.LLMSideQuery{Store: store, Chat: nil}
	sel, err := side.SelectForTurn(ctx, scope, "随便问", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel) == 0 {
		t.Fatal("expected recent fallback")
	}
}

func TestNormalizeEpisodeKindDemotesPersonality(t *testing.T) {
	if memory.NormalizeEpisodeKind("personality") != memory.EpisodeStateObservation {
		t.Fatal("expected demotion")
	}
}

func TestResolveEpisodeSubjectsSelf(t *testing.T) {
	scope := identity.LocalCLI()
	aboutSelf, ids := memory.ResolveEpisodeSubjects(scope, memory.EpisodeSelfNote, "")
	if !aboutSelf || len(ids) != 1 || ids[0] != "self" {
		t.Fatalf("self_note default: aboutSelf=%v ids=%v", aboutSelf, ids)
	}
	aboutSelf, ids = memory.ResolveEpisodeSubjects(scope, memory.EpisodeEvent, "myself")
	if !aboutSelf || ids[0] != "self" {
		t.Fatalf("about=myself: aboutSelf=%v ids=%v", aboutSelf, ids)
	}
	aboutSelf, ids = memory.ResolveEpisodeSubjects(scope, memory.EpisodeEvent, "")
	if aboutSelf || len(ids) != 1 || ids[0] != scope.PersonID() {
		t.Fatalf("event default person: aboutSelf=%v ids=%v", aboutSelf, ids)
	}
}

func TestNormalizeExperienceModeDefault(t *testing.T) {
	if memory.NormalizeExperienceMode("") != memory.ExperienceRealInteraction {
		t.Fatal("empty should default to real_interaction")
	}
	if memory.NormalizeExperienceMode("simulated_roleplay") != memory.ExperienceSimulatedRoleplay {
		t.Fatal("roleplay")
	}
}

func TestEpisodeExperienceModeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewEpisodeStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	ep, err := store.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{
		Kind: memory.EpisodeSelfNote, ExperienceMode: memory.ExperienceSimulatedRoleplay,
		Content: "角色代入中的判断", PersonIDs: []string{"self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), ep.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ExperienceMode != memory.ExperienceSimulatedRoleplay {
		t.Fatalf("got %q", got.ExperienceMode)
	}
}
