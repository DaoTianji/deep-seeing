package graph_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/origin"
)

func TestNeo4jIntegrationOriginSeed(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	_ = godotenv.Overload(filepath.Join("..", "..", ".env"))
	_ = godotenv.Overload(filepath.Join("..", "..", ".env.local"))
	if os.Getenv("NEO4J_PASSWORD") == "" {
		t.Skip("NEO4J_PASSWORD not set")
	}
	ctx := context.Background()
	store, err := graph.OpenFromEnv(ctx)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if store == nil {
		t.Skip("graph disabled")
	}
	defer store.Close(ctx)

	scope := identity.TenantScope{
		UserID:  "mudnet",
		AgentID: "deep-seeing-test-" + time.Now().Format("150405.000"),
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureOriginSeed(ctx, scope, origin.RoleAtOrigin); err != nil {
		t.Fatal(err)
	}
	bond, err := store.GetBond(ctx, scope, scope.PersonID())
	if err != nil {
		t.Fatal(err)
	}
	if bond.RoleAtOrigin != origin.RoleAtOrigin {
		t.Fatalf("role=%q", bond.RoleAtOrigin)
	}
	if bond.Basics != "" || bond.Strategy != "" {
		t.Fatalf("bond must start empty, got basics=%q strategy=%q", bond.Basics, bond.Strategy)
	}

	_, err = store.PatchBond(ctx, scope, "", graph.BondPatch{
		Basics: "最早的朋友之一", Style: "坦诚", StyleMode: "append",
	})
	if err != nil {
		t.Fatal(err)
	}
	bond2, err := store.PatchBond(ctx, scope, "", graph.BondPatch{
		Style: "整段覆盖", StyleMode: "replace",
	})
	if err != nil {
		t.Fatal(err)
	}
	text, _ := graph.FormatCompactRecall(bond2, "")
	if !strings.Contains(text, "坦诚") || !strings.Contains(text, "整段覆盖") {
		t.Fatalf("expected appended interaction items in compact: %s", text)
	}

	epID := "ep_test_" + time.Now().Format("150405")
	err = store.UpsertEpisodePointer(ctx, scope, graph.EpisodePointer{
		ID: epID, Kind: "event", Summary: "integration", DocURI: "by_id/" + epID + ".md",
		CreatedAt: time.Now().UTC(), PersonIDs: []string{scope.PersonID()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertCalls(ctx, scope, "", "安", epID); err != nil {
		t.Fatal(err)
	}
	bond, err = store.GetBond(ctx, scope, "")
	if err != nil {
		t.Fatal(err)
	}
	if bond.CallName != "安" {
		t.Fatalf("call=%q", bond.CallName)
	}
}
