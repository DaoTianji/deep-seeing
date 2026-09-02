package theater

import (
	"context"
	"errors"
	"testing"

	"deep-seeing/internal/identity"
)

func TestInitializationLifecycleBudgetAndRecovery(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	role, err := store.CreateDefinition(ctx, scope, RoleDefinitionWrite{DisplayName: "受控人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.CreateInitialization(ctx, role.ID, "建立成熟期人物", false, "duckduckgo")
	if err != nil || run.Status != InitPlanning || run.RemoteBudget != 24 {
		t.Fatalf("create: %#v %v", run, err)
	}
	plan := RoleResearchPlan{
		TargetPeriod: "成熟期",
		Questions:    []ResearchQuestion{{ID: "q1", Question: "他如何理解自己？"}},
	}
	run, err = store.SaveResearchPlan(ctx, run.ID, plan)
	if err != nil || run.Status != InitAwaitingPlanApproval {
		t.Fatalf("plan: %#v %v", run, err)
	}
	run, err = store.ApproveResearchPlan(ctx, run.ID)
	if err != nil || run.Status != InitCollecting || run.Plan.ApprovedAt == nil {
		t.Fatalf("approve: %#v %v", run, err)
	}
	for i := 0; i < DefaultInitializationRemoteBudget; i++ {
		if _, err = store.ConsumeInitializationRemote(ctx, run.ID); err != nil {
			t.Fatal(err)
		}
	}
	run, err = store.ConsumeInitializationRemote(ctx, run.ID)
	if !errors.Is(err, ErrInitializationBudget) || run.Status != InitNeedsBudget {
		t.Fatalf("budget: %#v %v", run, err)
	}
	run, err = store.GrantInitializationBudget(ctx, run.ID)
	if err != nil || run.Status != InitCollecting || run.RemoteBudget != 36 {
		t.Fatalf("grant: %#v %v", run, err)
	}
	recovered, err := store.RecoverInitializations(ctx)
	if err != nil || len(recovered) != 1 || recovered[0].Status != InitPaused || recovered[0].ResumeStatus != InitCollecting {
		t.Fatalf("recover: %#v %v", recovered, err)
	}
	run, err = store.ResumeInitialization(ctx, run.ID)
	if err != nil || run.Status != InitCollecting {
		t.Fatalf("resume: %#v %v", run, err)
	}
	if run.CurrentStep != "collect_sources" {
		t.Fatalf("resume kept stale recovery step: %q", run.CurrentStep)
	}
}

func TestNewEvidenceAfterBlueprintReturnsToAnalysis(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	role, _ := store.CreateDefinition(ctx, identity.LocalCLI(), RoleDefinitionWrite{DisplayName: "补充资料人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	run, _ := store.CreateInitialization(ctx, role.ID, "补充资料", false, "fixture")
	run, _ = store.SaveResearchPlan(ctx, run.ID, RoleResearchPlan{TargetPeriod: "成熟期", Questions: []ResearchQuestion{{ID: "q", Question: "问题"}}})
	run, _ = store.ApproveResearchPlan(ctx, run.ID)
	run, _ = store.TransitionInitialization(ctx, run.ID, InitAnalyzing, "coverage", "sources_collected", "")
	run, _ = store.TransitionInitialization(ctx, run.ID, InitCompiling, "compile", "coverage_analyzed", "")
	run, _ = store.TransitionInitialization(ctx, run.ID, InitBlueprinting, "blueprint", "compiled", "")
	run, err = store.SaveSourceAssessment(ctx, run.ID, SourceAssessment{SourceID: "new-source", Tier: SourceBiography, Audience: SourceActor, Status: AssessmentAccepted, ReadChunkIDs: []string{"new-chunk"}})
	if err != nil || run.Status != InitPaused || run.ResumeStatus != InitAnalyzing || run.CurrentStep != "sources_updated" || run.Checkpoint != "sources_updated" {
		t.Fatalf("new evidence did not invalidate analysis: %#v %v", run, err)
	}
}

func TestRetryInitializationRestoresFailedStage(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	role, _ := store.CreateDefinition(ctx, identity.LocalCLI(), RoleDefinitionWrite{DisplayName: "重试人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	run, _ := store.CreateInitialization(ctx, role.ID, "重试", false, "fixture")
	run, _ = store.SaveResearchPlan(ctx, run.ID, RoleResearchPlan{TargetPeriod: "成熟期", Questions: []ResearchQuestion{{ID: "q", Question: "问题"}}})
	run, _ = store.ApproveResearchPlan(ctx, run.ID)
	run, _ = store.TransitionInitialization(ctx, run.ID, InitAnalyzing, "coverage", "sources_collected", "")
	run, _ = store.TransitionInitialization(ctx, run.ID, InitCompiling, "compile", "coverage_analyzed", "")
	run, _ = store.TransitionInitialization(ctx, run.ID, InitBlueprinting, "blueprint", "compiled", "")
	run, _ = store.TransitionInitialization(ctx, run.ID, InitFailed, "blueprint", "compiled", "invalid JSON")
	run, err = store.RetryInitialization(ctx, run.ID)
	if err != nil || run.Status != InitBlueprinting || run.ErrorSummary != "" {
		t.Fatalf("retry did not restore blueprint stage: %#v %v", run, err)
	}
}

func TestInitializationModeParsing(t *testing.T) {
	if ParseInitializationMode("") != InitModeOff || ParseInitializationMode("OBSERVE") != InitModeObserve || ParseInitializationMode("agent") != InitModeAgent || ParseInitializationMode("bad") != InitModeOff {
		t.Fatal("unexpected initialization mode parsing")
	}
}

func TestListInitializationsReturnsEmptyCollection(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runs, err := store.ListInitializations(context.Background(), "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if runs == nil || len(runs) != 0 {
		t.Fatalf("expected a non-nil empty collection, got %#v", runs)
	}
}

func TestCritiqueHardIssuesCannotPass(t *testing.T) {
	issues := []CritiqueIssue{{Code: "anachronism", Severity: CritiqueHard}}
	if !hasUnresolvedHardIssue(issues) {
		t.Fatal("hard issue should block")
	}
	issues[0].Resolved = true
	if hasUnresolvedHardIssue(issues) {
		t.Fatal("resolved issue should not block")
	}
}
