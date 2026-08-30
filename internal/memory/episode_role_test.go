package memory

import (
	"context"
	"testing"

	"deep-seeing/internal/identity"
)

func TestEpisodeSearchSeparatesRoleNamespaces(t *testing.T) {
	ctx := context.Background()
	scope := identity.TenantScope{UserID: "u", AgentID: "a"}
	store, err := NewEpisodeStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "共同暗号", ExperienceMode: ExperienceRealInteraction})
	role, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{
		Content: "共同暗号", ExperienceMode: ExperienceSimulatedRoleplay,
		RoleID: "role_a", RoleInstanceID: "inst_a", RoleSessionID: "sess_a", WorldlineID: "world_a", RoleMemoryClass: "simulated",
	})
	other, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{
		Content: "共同暗号", ExperienceMode: ExperienceSimulatedRoleplay,
		RoleID: "role_b", RoleInstanceID: "inst_b", WorldlineID: "world_b", RoleMemoryClass: "simulated",
	})
	normal, err := store.Search(ctx, scope, Query{Text: "共同暗号", Limit: 10})
	if err != nil || len(normal) != 1 || normal[0].ID != real.ID {
		t.Fatalf("normal=%#v err=%v", normal, err)
	}
	actor, err := store.Search(ctx, scope, Query{Text: "共同暗号", Limit: 10, IncludeRole: true, RoleID: "role_a", RoleInstanceID: "inst_a", WorldlineID: "world_a"})
	if err != nil || len(actor) != 1 || actor[0].ID != role.ID || actor[0].ID == other.ID {
		t.Fatalf("actor=%#v err=%v", actor, err)
	}
	roundTrip, err := store.Get(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.RoleSessionID != "sess_a" || roundTrip.RoleMemoryClass != "simulated" {
		t.Fatalf("role metadata lost: %#v", roundTrip)
	}
}

func TestLegacyRoleplayExcludedFromNormalRecall(t *testing.T) {
	ctx := context.Background()
	scope := identity.TenantScope{UserID: "u", AgentID: "a"}
	store, _ := NewEpisodeStore(t.TempDir())
	_, _ = store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "旧扮演", ExperienceMode: ExperienceSimulatedRoleplay})
	got, err := store.Search(ctx, scope, Query{Text: "旧扮演", Limit: 10})
	if err != nil || len(got) != 0 {
		t.Fatalf("legacy roleplay leaked: %#v %v", got, err)
	}
}
