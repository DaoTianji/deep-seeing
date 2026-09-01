package theater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	DefaultInitializationRemoteBudget = 24
	InitializationBudgetExtension     = 12
)

var ErrInitializationBudget = errors.New("role initialization remote budget exhausted")

func (s *Store) CreateInitialization(ctx context.Context, roleID, objective string, privateConsent bool, provider string) (RoleInitializationRun, error) {
	d, err := s.GetDefinition(ctx, roleID)
	if err != nil {
		return RoleInitializationRun{}, err
	}
	if d.Status == DefinitionArchived || d.Status == DefinitionReady {
		return RoleInitializationRun{}, fmt.Errorf("role initialization requires a draft role")
	}
	now := time.Now().UTC()
	run := RoleInitializationRun{
		ID: "rinit_" + compactUUID(), RoleID: d.ID, VariantOfRoleID: d.VariantOfRoleID,
		Status: InitPlanning, CurrentStep: "research_plan", Objective: cleanText(objective),
		RemoteBudget: DefaultInitializationRemoteBudget, PrivateModelConsent: privateConsent,
		SearchProvider: cleanText(provider), Version: 1, CreatedAt: now, UpdatedAt: now,
		Coverage: CoverageMatrix{Items: defaultCoverageItems(), UpdatedAt: now},
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeJSONAtomic(s.initializationPath(run.ID), run); err != nil {
		return RoleInitializationRun{}, err
	}
	return run, nil
}

func defaultCoverageItems() []CoverageItem {
	dimensions := []string{"biography", "ideas", "relationships", "voice", "historical_context", "controversies", "unknowns"}
	out := make([]CoverageItem, 0, len(dimensions))
	for _, dimension := range dimensions {
		out = append(out, CoverageItem{Dimension: dimension, State: CoverageMissing})
	}
	return out
}

func (s *Store) GetInitialization(_ context.Context, id string) (RoleInitializationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var run RoleInitializationRun
	if err := readJSON(s.initializationPath(id), &run); err != nil {
		return RoleInitializationRun{}, err
	}
	return run, nil
}

func (s *Store) ListInitializations(_ context.Context, roleID string, limit int) ([]RoleInitializationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, err := filepath.Glob(filepath.Join(s.root, "initializations", "*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]RoleInitializationRun, 0)
	for _, path := range paths {
		var run RoleInitializationRun
		if readJSON(path, &run) != nil || (cleanText(roleID) != "" && run.RoleID != cleanText(roleID)) {
			continue
		}
		out = append(out, run)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) updateInitialization(_ context.Context, id string, expectedVersion int64, mutate func(*RoleInitializationRun) error) (RoleInitializationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var run RoleInitializationRun
	if err := readJSON(s.initializationPath(id), &run); err != nil {
		return RoleInitializationRun{}, err
	}
	if expectedVersion > 0 && run.Version != expectedVersion {
		return RoleInitializationRun{}, fmt.Errorf("initialization version conflict: have %d want %d", run.Version, expectedVersion)
	}
	if err := mutate(&run); err != nil {
		return RoleInitializationRun{}, err
	}
	run.Version++
	run.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.initializationPath(id), run); err != nil {
		return RoleInitializationRun{}, err
	}
	return run, nil
}

func (s *Store) SaveResearchPlan(ctx context.Context, id string, plan RoleResearchPlan) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status != InitPlanning && run.Status != InitDraft {
			return fmt.Errorf("research plan cannot be saved from %s", run.Status)
		}
		plan.TargetPeriod = cleanText(plan.TargetPeriod)
		plan.KnowledgeCutoff = cleanText(plan.KnowledgeCutoff)
		if plan.TargetPeriod == "" || len(plan.Questions) == 0 {
			return fmt.Errorf("target period and research questions required")
		}
		if plan.CreatedAt.IsZero() {
			plan.CreatedAt = time.Now().UTC()
		}
		run.Plan = &plan
		run.Status = InitAwaitingPlanApproval
		run.CurrentStep = "awaiting_plan_approval"
		run.Checkpoint = "plan_ready"
		return nil
	})
}

func (s *Store) ApproveResearchPlan(ctx context.Context, id string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status != InitAwaitingPlanApproval || run.Plan == nil {
			return fmt.Errorf("research plan is not awaiting approval")
		}
		now := time.Now().UTC()
		run.Plan.ApprovedAt = &now
		run.Status = InitCollecting
		run.CurrentStep = "collect_sources"
		run.Checkpoint = "plan_approved"
		return nil
	})
}

func (s *Store) TransitionInitialization(ctx context.Context, id string, status InitializationStatus, step, checkpoint, errorSummary string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if !validInitializationTransition(run.Status, status) {
			return fmt.Errorf("invalid initialization transition %s -> %s", run.Status, status)
		}
		run.Status = status
		run.CurrentStep = cleanText(step)
		run.Checkpoint = cleanText(checkpoint)
		run.ErrorSummary = truncateActionText(errorSummary, 240)
		if status == InitCompleted {
			now := time.Now().UTC()
			run.CompletedAt = &now
		}
		return nil
	})
}

func validInitializationTransition(from, to InitializationStatus) bool {
	if from == to {
		return true
	}
	if to == InitPaused || to == InitFailed || to == InitCancelled || to == InitNeedsBudget {
		return from != InitCompleted && from != InitCancelled
	}
	allowed := map[InitializationStatus][]InitializationStatus{
		InitDraft: {InitPlanning}, InitPlanning: {InitAwaitingPlanApproval},
		InitAwaitingPlanApproval: {InitCollecting}, InitCollecting: {InitAnalyzing},
		InitAnalyzing: {InitCompiling}, InitCompiling: {InitBlueprinting},
		InitBlueprinting: {InitCritiquing}, InitCritiquing: {InitAwaitingFinalApproval, InitBlueprinting},
		InitAwaitingFinalApproval: {InitCompleted, InitBlueprinting}, InitPaused: {InitPlanning, InitCollecting, InitAnalyzing, InitCompiling, InitBlueprinting, InitCritiquing, InitAwaitingFinalApproval},
		InitNeedsBudget: {InitCollecting},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

func (s *Store) ConsumeInitializationRemote(ctx context.Context, id string) (RoleInitializationRun, error) {
	var exhausted bool
	run, err := s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.RemoteUsed >= run.RemoteBudget {
			run.ResumeStatus = InitCollecting
			run.Status = InitNeedsBudget
			run.CurrentStep = "budget_required"
			exhausted = true
			return nil
		}
		run.RemoteUsed++
		return nil
	})
	if err != nil {
		return RoleInitializationRun{}, err
	}
	if exhausted {
		return run, ErrInitializationBudget
	}
	return run, nil
}

func (s *Store) GrantInitializationBudget(ctx context.Context, id string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		run.RemoteBudget += InitializationBudgetExtension
		if run.Status == InitNeedsBudget {
			run.Status = InitCollecting
			run.ResumeStatus = ""
			run.CurrentStep = "collect_sources"
		}
		return nil
	})
}

func (s *Store) PauseInitialization(ctx context.Context, id string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status == InitCompleted || run.Status == InitCancelled {
			return fmt.Errorf("completed initialization cannot be paused")
		}
		run.ResumeStatus = run.Status
		run.Status = InitPaused
		run.CurrentStep = "paused"
		return nil
	})
}

func (s *Store) ResumeInitialization(ctx context.Context, id string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status != InitPaused {
			return fmt.Errorf("initialization is not paused")
		}
		if run.ResumeStatus == "" {
			run.ResumeStatus = InitPlanning
		}
		run.Status = run.ResumeStatus
		run.ResumeStatus = ""
		return nil
	})
}

func (s *Store) RetryInitialization(ctx context.Context, id string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status != InitFailed {
			return fmt.Errorf("initialization is not failed")
		}
		target := InitCollecting
		switch run.CurrentStep {
		case "research_plan":
			target = InitPlanning
		case "collect_sources":
			target = InitCollecting
		case "coverage":
			target = InitAnalyzing
		case "compile":
			target = InitCompiling
		case "blueprint":
			target = InitBlueprinting
		case "critic":
			target = InitCritiquing
		}
		if run.Plan == nil {
			target = InitPlanning
		}
		run.Status = target
		run.ResumeStatus = ""
		run.ErrorSummary = ""
		run.CurrentStep = string(target)
		return nil
	})
}

func (s *Store) PauseInitializationForEvidence(ctx context.Context, id string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status != InitBlueprinting && run.Status != InitCompiling && run.Status != InitAnalyzing {
			return fmt.Errorf("evidence pause is not valid from %s", run.Status)
		}
		run.ResumeStatus = InitAnalyzing
		run.Status = InitPaused
		run.CurrentStep = "awaiting_sources"
		run.Checkpoint = "evidence_required"
		run.ErrorSummary = "没有已读取并采用的来源，已暂停塑造；请补充资料或改善搜索覆盖后恢复"
		return nil
	})
}

func (s *Store) CancelInitialization(ctx context.Context, id string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status == InitCompleted {
			return fmt.Errorf("completed initialization cannot be cancelled")
		}
		run.Status = InitCancelled
		run.CurrentStep = "cancelled"
		return nil
	})
}

func (s *Store) AppendInitializationEvent(ctx context.Context, id string, event RoleInitializationEvent) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if event.At.IsZero() {
			event.At = time.Now().UTC()
		}
		if event.RunID == "" {
			event.RunID = run.ID
		}
		if event.RoleID == "" {
			event.RoleID = run.RoleID
		}
		run.Events = append(run.Events, event)
		if len(run.Events) > 500 {
			run.Events = run.Events[len(run.Events)-500:]
		}
		return nil
	})
}

func (s *Store) SaveSourceAssessment(ctx context.Context, id string, assessment SourceAssessment) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		assessment.SourceID = cleanText(assessment.SourceID)
		assessment.Tier = normalizeSourceTier(assessment.Tier)
		if assessment.SourceID == "" {
			return fmt.Errorf("source_id required")
		}
		if assessment.Audience == "" {
			assessment.Audience = SourceActor
		}
		if assessment.UpdatedAt.IsZero() {
			assessment.UpdatedAt = time.Now().UTC()
		}
		replaced := false
		for i := range run.Assessments {
			if run.Assessments[i].SourceID == assessment.SourceID {
				run.Assessments[i] = assessment
				replaced = true
				break
			}
		}
		if !replaced {
			run.Assessments = append(run.Assessments, assessment)
		}
		switch run.Status {
		case InitBlueprinting, InitCritiquing, InitAwaitingFinalApproval:
			run.Status = InitAnalyzing
			run.CurrentStep = "coverage"
			run.Checkpoint = "sources_updated"
			run.ErrorSummary = ""
		case InitPaused:
			run.ResumeStatus = InitAnalyzing
		}
		return nil
	})
}

func (s *Store) SaveCoverage(ctx context.Context, id string, coverage CoverageMatrix, conflicts []EvidenceConflict) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if coverage.UpdatedAt.IsZero() {
			coverage.UpdatedAt = time.Now().UTC()
		}
		run.Coverage = coverage
		run.Conflicts = conflicts
		return nil
	})
}

func (s *Store) RequestBlueprintRevision(ctx context.Context, id, reason string) (RoleInitializationRun, error) {
	return s.updateInitialization(ctx, id, 0, func(run *RoleInitializationRun) error {
		if run.Status != InitAwaitingFinalApproval && run.Status != InitBlueprinting {
			return fmt.Errorf("blueprint revision cannot be requested from %s", run.Status)
		}
		run.RevisionRequest = truncateActionText(reason, 500)
		run.Status = InitBlueprinting
		run.CurrentStep = "blueprint_revision"
		run.Checkpoint = "revision_requested"
		return nil
	})
}

func (s *Store) AcceptCritiqueWarnings(ctx context.Context, id, reason string) (RoleCritique, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var critique RoleCritique
	if err := readJSON(s.critiquePath(id), &critique); err != nil {
		return RoleCritique{}, err
	}
	if hasUnresolvedHardIssue(critique.Issues) {
		return RoleCritique{}, fmt.Errorf("critic hard errors cannot be accepted")
	}
	if cleanText(reason) == "" {
		return RoleCritique{}, fmt.Errorf("warning acceptance reason required")
	}
	now := time.Now().UTC()
	critique.WarningAcceptanceReason = truncateActionText(reason, 500)
	critique.ApprovedAt = &now
	if err := writeJSONAtomic(s.critiquePath(id), critique); err != nil {
		return RoleCritique{}, err
	}
	return critique, nil
}

func (s *Store) SaveBlueprint(ctx context.Context, blueprint RoleBlueprint) (RoleBlueprint, RoleInitializationRun, error) {
	run, err := s.GetInitialization(ctx, blueprint.RunID)
	if err != nil {
		return RoleBlueprint{}, RoleInitializationRun{}, err
	}
	if run.RoleID != blueprint.RoleID {
		return RoleBlueprint{}, RoleInitializationRun{}, fmt.Errorf("blueprint role mismatch")
	}
	if blueprint.ID == "" {
		blueprint.ID = "rblue_" + compactUUID()
	}
	if blueprint.Version <= 0 {
		blueprint.Version = 1
	}
	if blueprint.CreatedAt.IsZero() {
		blueprint.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	if err := writeJSONAtomic(s.blueprintPath(blueprint.ID), blueprint); err != nil {
		s.mu.Unlock()
		return RoleBlueprint{}, RoleInitializationRun{}, err
	}
	s.mu.Unlock()
	updated, err := s.updateInitialization(ctx, run.ID, run.Version, func(value *RoleInitializationRun) error {
		value.BlueprintID = blueprint.ID
		return nil
	})
	return blueprint, updated, err
}

func (s *Store) GetBlueprint(_ context.Context, id string) (RoleBlueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var value RoleBlueprint
	err := readJSON(s.blueprintPath(id), &value)
	return value, err
}

func (s *Store) SaveCritique(ctx context.Context, critique RoleCritique) (RoleCritique, RoleInitializationRun, error) {
	run, err := s.GetInitialization(ctx, critique.RunID)
	if err != nil {
		return RoleCritique{}, RoleInitializationRun{}, err
	}
	if run.BlueprintID == "" || run.BlueprintID != critique.BlueprintID {
		return RoleCritique{}, RoleInitializationRun{}, fmt.Errorf("critique blueprint mismatch")
	}
	if critique.ID == "" {
		critique.ID = "rcrit_" + compactUUID()
	}
	if critique.CreatedAt.IsZero() {
		critique.CreatedAt = time.Now().UTC()
	}
	critique.Passed = !hasUnresolvedHardIssue(critique.Issues)
	s.mu.Lock()
	if err := writeJSONAtomic(s.critiquePath(critique.ID), critique); err != nil {
		s.mu.Unlock()
		return RoleCritique{}, RoleInitializationRun{}, err
	}
	s.mu.Unlock()
	updated, err := s.updateInitialization(ctx, run.ID, run.Version, func(value *RoleInitializationRun) error {
		value.CritiqueID = critique.ID
		return nil
	})
	return critique, updated, err
}

func (s *Store) GetCritique(_ context.Context, id string) (RoleCritique, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var value RoleCritique
	err := readJSON(s.critiquePath(id), &value)
	return value, err
}

func hasUnresolvedHardIssue(issues []CritiqueIssue) bool {
	for _, issue := range issues {
		if issue.Severity == CritiqueHard && !issue.Resolved {
			return true
		}
	}
	return false
}

func (s *Store) RecoverInitializations(ctx context.Context) ([]RoleInitializationRun, error) {
	runs, err := s.ListInitializations(ctx, "", 0)
	if err != nil {
		return nil, err
	}
	var changed []RoleInitializationRun
	for _, run := range runs {
		if !initializationRunning(run.Status) {
			continue
		}
		updated, updateErr := s.updateInitialization(ctx, run.ID, run.Version, func(value *RoleInitializationRun) error {
			value.ResumeStatus = value.Status
			value.Status = InitPaused
			value.CurrentStep = "process_recovery"
			return nil
		})
		if updateErr != nil {
			return changed, updateErr
		}
		changed = append(changed, updated)
	}
	return changed, nil
}

func initializationRunning(status InitializationStatus) bool {
	switch status {
	case InitPlanning, InitCollecting, InitAnalyzing, InitCompiling, InitBlueprinting, InitCritiquing:
		return true
	default:
		return false
	}
}

func (s *Store) initializationPath(id string) string {
	return filepath.Join(s.root, "initializations", safeID(id)+".json")
}
func (s *Store) blueprintPath(id string) string {
	return filepath.Join(s.root, "blueprints", safeID(id)+".json")
}
func (s *Store) critiquePath(id string) string {
	return filepath.Join(s.root, "critiques", safeID(id)+".json")
}

func initializationExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
