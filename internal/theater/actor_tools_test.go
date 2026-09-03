package theater

import (
	"context"
	"path/filepath"
	"strings"
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

func TestActorBookToolsReadOnlyActorVisibleCorpus(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	corpus, err := NewCorpusStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	actorDoc, actorChunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: "role", CorpusRoleID: "corpus", SourceID: "actor-source", Title: "原书", Audience: SourceActor, Tier: SourcePrimary, Text: []byte("community feeling guides the argument")})
	if err != nil || actorDoc.ID == "" || len(actorChunks) != 1 {
		t.Fatal(err)
	}
	_, directorChunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: "role", CorpusRoleID: "corpus", SourceID: "director-source", Title: "后世评论", Audience: SourceDirector, Tier: SourcePosthumous, Text: []byte("hidden modern assessment")})
	if err != nil || len(directorChunks) != 1 {
		t.Fatal(err)
	}
	episodes, _ := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	all, err := ActorTools(ActorToolContext{Scope: testScope(), Episodes: episodes, Corpus: corpus, Definition: RoleDefinition{ID: "role", CorpusRoleID: "corpus", Kind: RoleCharacter}, Instance: RoleInstance{ID: "inst"}, Session: RoleSession{ID: "session", WorldlineID: "world"}, Worldlines: []RoleWorldline{{ID: "world"}}})
	if err != nil {
		t.Fatal(err)
	}
	search := findTheaterInvokable(t, all, "search_book_passages")
	result, err := search.InvokableRun(ctx, `{"query":"community"}`)
	if err != nil || !strings.Contains(result, actorChunks[0].ID) || strings.Contains(result, directorChunks[0].ID) || strings.Contains(result, "hidden modern assessment") {
		t.Fatalf("candidate isolation failed: %s %v", result, err)
	}
	read := findTheaterInvokable(t, all, "read_book_passage")
	result, err = read.InvokableRun(ctx, `{"id":"`+actorChunks[0].ID+`"}`)
	if err != nil || !strings.Contains(result, "guides the argument") {
		t.Fatalf("actor passage unavailable: %s %v", result, err)
	}
	result, err = read.InvokableRun(ctx, `{"id":"`+directorChunks[0].ID+`"}`)
	if err != nil || !strings.Contains(result, "outside current role corpus") || strings.Contains(result, "modern assessment") {
		t.Fatalf("director-only passage leaked: %s %v", result, err)
	}
}

func findTheaterInvokable(t *testing.T, all []einotool.BaseTool, name string) einotool.InvokableTool {
	t.Helper()
	for _, candidate := range all {
		info, err := candidate.Info(context.Background())
		if err == nil && info.Name == name {
			invokable, ok := candidate.(einotool.InvokableTool)
			if !ok {
				t.Fatalf("tool %s is not invokable", name)
			}
			return invokable
		}
	}
	t.Fatalf("tool %s not found", name)
	return nil
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
