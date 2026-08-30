package theater

import (
	"context"
	"path/filepath"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"

	"deep-seeing/internal/memory"
	"deep-seeing/internal/workspace"
)

func TestProfessionalToolsRequireExplicitPolicy(t *testing.T) {
	episodes, _ := memory.NewEpisodeStore(filepath.Join(t.TempDir(), "episodes"))
	ws, _ := workspace.NewStore(filepath.Join(t.TempDir(), "workspace"))
	base := ActorToolContext{
		Scope: testScope(), Episodes: episodes, Workspace: ws,
		Definition: RoleDefinition{ID: "editor", Kind: RoleProfessional},
		Instance:   RoleInstance{ID: "inst"}, Session: RoleSession{ID: "session", WorldlineID: "world"},
		Worldlines: []RoleWorldline{{ID: "world"}},
	}
	without, err := ActorTools(base)
	if err != nil {
		t.Fatal(err)
	}
	if names := theaterToolNames(t, without); names["read_workspace"] || names["write_workspace"] {
		t.Fatalf("professional tools leaked without policy: %#v", names)
	}
	base.Definition.ToolPolicy.Allowed = []string{"read_workspace", "write_workspace"}
	with, err := ActorTools(base)
	if err != nil {
		t.Fatal(err)
	}
	names := theaterToolNames(t, with)
	if !names["read_workspace"] || !names["write_workspace"] || names["list_workspace"] {
		t.Fatalf("wrong whitelist: %#v", names)
	}
}

func TestCharacterNeverReceivesProfessionalTools(t *testing.T) {
	episodes, _ := memory.NewEpisodeStore(filepath.Join(t.TempDir(), "episodes"))
	ws, _ := workspace.NewStore(filepath.Join(t.TempDir(), "workspace"))
	all, err := ActorTools(ActorToolContext{
		Scope: testScope(), Episodes: episodes, Workspace: ws,
		Definition: RoleDefinition{ID: "actor", Kind: RoleCharacter, ToolPolicy: RoleToolPolicy{Allowed: []string{"write_workspace"}}},
		Instance:   RoleInstance{ID: "inst"}, Session: RoleSession{ID: "session", WorldlineID: "world"},
		Worldlines: []RoleWorldline{{ID: "world"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if theaterToolNames(t, all)["write_workspace"] {
		t.Fatal("character received professional workspace write tool")
	}
}

func theaterToolNames(t *testing.T, tools []einotool.BaseTool) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, candidate := range tools {
		info, err := candidate.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		out[info.Name] = true
	}
	return out
}
