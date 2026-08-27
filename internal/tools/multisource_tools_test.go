package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/workspace"
)

func TestSceneAndProposalCandidateReadUseLifecycle(t *testing.T) {
	root := t.TempDir()
	scope := identity.LocalCLI()
	episodes, err := memory.NewEpisodeStore(root + "/episodes")
	if err != nil {
		t.Fatal(err)
	}
	scenes, err := memory.NewSceneStore(root + "/scenes")
	if err != nil {
		t.Fatal(err)
	}
	scene, err := scenes.Write(scope, memory.SceneNorm{
		Title: "发布复盘", Keywords: []string{"发布", "复盘"},
		Body: "SECRET_SCENE_BODY 先核对影响范围，再讨论责任。",
	})
	if err != nil {
		t.Fatal(err)
	}
	proposals, err := memory.NewProposalStore(root + "/proposals")
	if err != nil {
		t.Fatal(err)
	}
	longProposal := "尚未确认：" + strings.Repeat("假设内容", 40) + " SECRET_PROPOSAL_TAIL"
	proposal, err := proposals.Enqueue(context.Background(), scope, memory.ProposalWrite{
		PersonID: scope.PersonID(), Field: "interaction", SuggestedText: longProposal,
		Rationale: "SECRET_RATIONALE", Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := All(Deps{
		Scope: scope, Episodes: episodes, Scenes: scenes, Proposals: proposals, RecallMode: "agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, collector := observe.WithContextHooks(context.Background(), observe.ContextHooks{})

	sceneList, err := findInvokableTool(t, all, "list_scene_norms").InvokableRun(ctx, `{"query":"发布"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sceneList, scene.ID) || !strings.Contains(sceneList, `"source":"scene_norm"`) ||
		strings.Contains(sceneList, "SECRET_SCENE_BODY") || strings.Contains(sceneList, `"body"`) {
		t.Fatalf("invalid scene candidates: %s", sceneList)
	}
	beforeRead, err := findInvokableTool(t, all, "report_context_use").InvokableRun(ctx,
		`{"source":"scene_norm","id":"`+scene.ID+`","disposition":"used"}`)
	if err != nil || !strings.Contains(beforeRead, `"ok":false`) {
		t.Fatalf("scene use before read accepted: %s err=%v", beforeRead, err)
	}
	if _, err := findInvokableTool(t, all, "read_scene_norm").InvokableRun(ctx, `{"id":"`+scene.ID+`"}`); err != nil {
		t.Fatal(err)
	}
	usedScene, err := findInvokableTool(t, all, "report_context_use").InvokableRun(ctx,
		`{"source":"scene_norm","id":"`+scene.ID+`","disposition":"used"}`)
	if err != nil || !strings.Contains(usedScene, `"ok":true`) {
		t.Fatalf("read scene use rejected: %s err=%v", usedScene, err)
	}

	proposalList, err := findInvokableTool(t, all, "list_proposals").InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proposalList, proposal.ID) || !strings.Contains(proposalList, `"role":"hypothesis"`) ||
		strings.Contains(proposalList, "SECRET_PROPOSAL_TAIL") || strings.Contains(proposalList, "SECRET_RATIONALE") {
		t.Fatalf("invalid proposal candidates: %s", proposalList)
	}
	if _, err := findInvokableTool(t, all, "read_proposal").InvokableRun(ctx, `{"id":"`+proposal.ID+`"}`); err != nil {
		t.Fatal(err)
	}
	usedProposal, err := findInvokableTool(t, all, "report_context_use").InvokableRun(ctx,
		`{"source":"proposal","id":"`+proposal.ID+`","disposition":"used"}`)
	if err != nil || !strings.Contains(usedProposal, `"ok":true`) {
		t.Fatalf("read proposal use rejected: %s err=%v", usedProposal, err)
	}

	if len(collector.Reads()) != 2 || len(collector.Uses()) != 2 {
		t.Fatalf("unexpected context trace: reads=%+v uses=%+v", collector.Reads(), collector.Uses())
	}
}

func TestMultiSourceToolsAreAgentOnlyAndRejectedProposalsStayHidden(t *testing.T) {
	root := t.TempDir()
	scope := identity.LocalCLI()
	episodes, _ := memory.NewEpisodeStore(root + "/episodes")
	scenes, _ := memory.NewSceneStore(root + "/scenes")
	proposals, _ := memory.NewProposalStore(root + "/proposals")
	legacyScene, err := scenes.Write(scope, memory.SceneNorm{
		Title: "Legacy scene", Keywords: []string{"legacy"}, Body: "LEGACY_SCENE_BODY",
	})
	if err != nil {
		t.Fatal(err)
	}

	rejected, err := proposals.Enqueue(context.Background(), scope, memory.ProposalWrite{
		PersonID: scope.PersonID(), Field: "baseline", SuggestedText: "待拒绝假设",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proposals.Resolve(context.Background(), rejected.ID, memory.ProposalRejected); err != nil {
		t.Fatal(err)
	}
	agentTools, err := All(Deps{Scope: scope, Episodes: episodes, Scenes: scenes, Proposals: proposals, RecallMode: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := findInvokableTool(t, agentTools, "list_proposals").InvokableRun(context.Background(), `{}`)
	if err != nil || strings.Contains(out, rejected.ID) {
		t.Fatalf("rejected proposal leaked: %s err=%v", out, err)
	}
	proposalOnly, err := All(Deps{Scope: scope, Episodes: episodes, Proposals: proposals, RecallMode: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	if names := toolNames(t, proposalOnly); !names["list_proposals"] || !names["read_proposal"] || !names["report_context_use"] {
		t.Fatalf("proposal tools unexpectedly depend on SceneNorm: %+v", names)
	}
	catalogOut, err := findInvokableTool(t, proposalOnly, "list_capabilities").InvokableRun(context.Background(), `{}`)
	if err != nil || !strings.Contains(catalogOut, `"name":"list_proposals"`) || !strings.Contains(catalogOut, `"name":"report_context_use"`) {
		t.Fatalf("agent capability catalog omitted proposal tools: %s err=%v", catalogOut, err)
	}

	legacyTools, err := All(Deps{Scope: scope, Episodes: episodes, Scenes: scenes, Proposals: proposals, RecallMode: "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	legacyCatalog, err := findInvokableTool(t, legacyTools, "list_capabilities").InvokableRun(context.Background(), `{}`)
	if err != nil || strings.Contains(legacyCatalog, `"name":"list_proposals"`) || strings.Contains(legacyCatalog, `"name":"report_context_use"`) {
		t.Fatalf("legacy capability catalog exposed agent tools: %s err=%v", legacyCatalog, err)
	}

	names := toolNames(t, legacyTools)
	for _, name := range []string{"list_proposals", "read_proposal", "report_context_use"} {
		if names[name] {
			t.Fatalf("legacy exposed experimental tool %s", name)
		}
	}
	legacySceneList, err := findInvokableTool(t, legacyTools, "list_scene_norms").InvokableRun(context.Background(), `{}`)
	if err != nil || !strings.Contains(legacySceneList, legacyScene.ID) ||
		!strings.Contains(legacySceneList, "LEGACY_SCENE_BODY") || !strings.Contains(legacySceneList, `"scenes"`) {
		t.Fatalf("legacy SceneNorm behavior changed: %s err=%v", legacySceneList, err)
	}
}

func TestLegacyListContractsRemainStable(t *testing.T) {
	root := t.TempDir()
	scope := identity.LocalCLI()
	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := episodes.WriteEpisode(context.Background(), scope, memory.EpisodeWrite{
		Kind: memory.EpisodeEvent, Content: "legacy marker happened",
	}); err != nil {
		t.Fatal(err)
	}
	workspaceStore, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceStore.Create(workspace.Write{
		Type: workspace.TypeProject, Title: "Legacy project", Summary: "legacy summary", Body: "legacy body",
	}); err != nil {
		t.Fatal(err)
	}
	intentStore, err := intent.OpenStore(filepath.Join(root, "intent"))
	if err != nil {
		t.Fatal(err)
	}
	defer intentStore.Close()
	if _, err := intentStore.Create(context.Background(), intent.CreateInput{
		AgentID: scope.AgentID, Kind: intent.IntentOneShot, Title: "Legacy intent",
		DueAt: time.Now().UTC().Add(time.Hour), Trigger: intent.TriggerSelfScheduled,
	}); err != nil {
		t.Fatal(err)
	}

	all, err := All(Deps{
		Scope: scope, Episodes: episodes, Workspace: workspaceStore, Intents: intentStore, RecallMode: "legacy",
	})
	if err != nil {
		t.Fatal(err)
	}
	workspaceOut, err := findInvokableTool(t, all, "list_workspace").InvokableRun(context.Background(), `{}`)
	if err != nil || !strings.Contains(workspaceOut, `"items"`) || !strings.Contains(workspaceOut, `"summary":"legacy summary"`) || strings.Contains(workspaceOut, `"candidates"`) {
		t.Fatalf("legacy workspace contract changed: %s err=%v", workspaceOut, err)
	}
	intentOut, err := findInvokableTool(t, all, "list_intents").InvokableRun(context.Background(), `{}`)
	if err != nil || !strings.Contains(intentOut, `"intents"`) || !strings.Contains(intentOut, `"due_at"`) || strings.Contains(intentOut, `"candidates"`) {
		t.Fatalf("legacy intent contract changed: %s err=%v", intentOut, err)
	}
	episodeOut, err := findInvokableTool(t, all, "search_episodes").InvokableRun(context.Background(), `{"query":"legacy marker"}`)
	if err != nil || !strings.Contains(episodeOut, `"summary":"legacy marker`) || strings.Contains(episodeOut, `"source":"episode"`) {
		t.Fatalf("legacy episode contract changed: %s err=%v", episodeOut, err)
	}
}

func toolNames(t *testing.T, tools []einotool.BaseTool) map[string]bool {
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
