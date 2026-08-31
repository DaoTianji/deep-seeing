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
