// Command rebuild-role re-runs an existing public role through whole-book
// reading in an isolated role-store copy. It never connects to or mutates the
// production role directory.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/theater"
)

type retryCompleter struct{ chat *memory.ChatClient }

func (r retryCompleter) Complete(ctx context.Context, system, user string) (string, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		value, err := r.chat.Complete(ctx, system, user)
		if err == nil {
			return value, nil
		}
		last = err
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
		}
	}
	return "", last
}

type rebuildReport struct {
	StartedAt       time.Time                     `json:"started_at"`
	CompletedAt     time.Time                     `json:"completed_at"`
	SourceRoot      string                        `json:"source_root"`
	WorkRoot        string                        `json:"work_root"`
	BaselineRole    theater.RoleDefinition        `json:"baseline_role"`
	BaselineClaims  []theater.RoleClaim           `json:"baseline_claims"`
	RebuiltRole     theater.RoleDefinition        `json:"rebuilt_role"`
	Initialization  theater.RoleInitializationRun `json:"initialization"`
	Blueprint       theater.RoleBlueprint         `json:"blueprint"`
	Critique        theater.RoleCritique          `json:"critique"`
	Claims          []theater.RoleClaim           `json:"claims"`
	Readings        []theater.BookReadingRun      `json:"readings"`
	Documents       int                           `json:"documents"`
	AvailableChunks int                           `json:"available_chunks"`
	ReadChunks      int                           `json:"read_chunks"`
	Usage           memory.ChatUsage              `json:"usage"`
	Error           string                        `json:"error,omitempty"`
}

func main() {
	var (
		sourceRoot = flag.String("source-root", "", "immutable baseline role-store root")
		workRoot   = flag.String("work-root", "", "isolated writable role-store root")
		roleID     = flag.String("role-id", "", "role definition id to rebuild")
		sourceRun  = flag.String("source-run", "", "completed initialization supplying the approved plan and assessments")
		resumeRun  = flag.String("resume-run", "", "existing failed initialization in work-root to resume from its checkpoint")
		outPath    = flag.String("out", "", "JSON report path")
	)
	flag.Parse()
	if *sourceRoot == "" || *workRoot == "" || *roleID == "" || *sourceRun == "" || *outPath == "" {
		log.Fatal("source-root, work-root, role-id, source-run and out are required")
	}
	if samePath(*sourceRoot, *workRoot) {
		log.Fatal("work-root must be an isolated copy, not source-root")
	}
	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")
	cfg := deepagent.ConfigFromEnv()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		log.Fatal("OPENAI_API_KEY and OPENAI_MODEL are required")
	}
	ctx := context.Background()
	report := rebuildReport{StartedAt: time.Now().UTC(), SourceRoot: *sourceRoot, WorkRoot: *workRoot}
	defer func() {
		report.CompletedAt = time.Now().UTC()
		if err := writeReport(*outPath, report); err != nil {
			log.Printf("write report: %v", err)
		}
	}()

	baseline, err := theater.NewStore(*sourceRoot)
	if err != nil {
		fail(&report, err)
	}
	report.BaselineRole, err = baseline.GetDefinition(ctx, *roleID)
	if err != nil {
		fail(&report, err)
	}
	report.BaselineClaims, _ = baseline.ListClaims(ctx, *roleID)
	previousRun, err := baseline.GetInitialization(ctx, *sourceRun)
	if err != nil || previousRun.RoleID != *roleID || previousRun.Plan == nil {
		fail(&report, fmt.Errorf("baseline initialization is incomplete: %w", err))
	}

	store, err := theater.NewStore(*workRoot)
	if err != nil {
		fail(&report, err)
	}
	corpus, err := theater.NewCorpusStore(*workRoot)
	if err != nil {
		fail(&report, err)
	}
	defer corpus.Close()
	if err := corpus.Rebuild(ctx); err != nil {
		fail(&report, fmt.Errorf("rebuild corpus index: %w", err))
	}
	role, err := store.GetDefinition(ctx, *roleID)
	if err != nil {
		fail(&report, err)
	}
	documents, err := corpus.ListDocuments(ctx, role.CorpusRoleID)
	if err != nil {
		fail(&report, err)
	}
	report.Documents = len(documents)
	var run theater.RoleInitializationRun
	if strings.TrimSpace(*resumeRun) != "" {
		run, err = store.GetInitialization(ctx, *resumeRun)
		if err != nil || run.RoleID != role.ID {
			fail(&report, fmt.Errorf("resume initialization is invalid: %w", err))
		}
		if run.Status == theater.InitFailed {
			run, err = store.RetryInitialization(ctx, run.ID)
			if err != nil {
				fail(&report, err)
			}
		}
		for _, assessment := range run.Assessments {
			report.AvailableChunks += len(assessment.AvailableChunkIDs)
		}
	} else {
		role.Identity, role.Voice, role.Timeline = "", "", nil
		role.CharacterModel, role.Validation = nil, nil
		role.BlueprintVersion, role.InitializationRunID, role.MainInstanceID = 0, "", ""
		role.Status = theater.DefinitionDraft
		role, err = store.SaveDefinition(ctx, role, role.Version)
		if err != nil {
			fail(&report, err)
		}
		run, err = store.CreateInitialization(ctx, role.ID, "按照整书阅读方法重构人物，并验证实际表演是否更丰富", false, "existing_public_corpus")
		if err != nil {
			fail(&report, err)
		}
		plan := *previousRun.Plan
		plan.ApprovedAt = nil
		plan.CreatedAt = time.Now().UTC()
		plan.Planner = "approved_plan_rebuild"
		run, err = store.SaveResearchPlan(ctx, run.ID, plan)
		if err != nil {
			fail(&report, err)
		}
		run, err = store.ApproveResearchPlan(ctx, run.ID)
		if err != nil {
			fail(&report, err)
		}
		for _, old := range previousRun.Assessments {
			if old.Status != theater.AssessmentAccepted || old.Tier == theater.SourceGenerated {
				continue
			}
			assessment := theater.SourceAssessment{SourceID: old.SourceID, Tier: old.Tier, Audience: old.Audience, Status: theater.AssessmentAccepted, Reliable: old.Reliable, ReasonCode: "whole_book_rebuild", UpdatedAt: time.Now().UTC()}
			for _, document := range documents {
				if document.SourceID != old.SourceID {
					continue
				}
				chunks, listErr := corpus.ListDocumentChunks(ctx, document.ID)
				if listErr != nil {
					fail(&report, listErr)
				}
				for _, chunk := range chunks {
					assessment.AvailableChunkIDs = append(assessment.AvailableChunkIDs, chunk.ID)
				}
			}
			if len(assessment.AvailableChunkIDs) == 0 {
				assessment.AvailableChunkIDs = append(assessment.AvailableChunkIDs, old.ReadChunkIDs...)
			}
			report.AvailableChunks += len(assessment.AvailableChunkIDs)
			run, err = store.SaveSourceAssessment(ctx, run.ID, assessment)
			if err != nil {
				fail(&report, err)
			}
		}
		run, err = store.TransitionInitialization(ctx, run.ID, theater.InitAnalyzing, "whole_book_reading", "existing_sources_registered", "")
		if err != nil {
			fail(&report, err)
		}
	}

	chat := &memory.ChatClient{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 8192, HTTPClient: &http.Client{Timeout: 240 * time.Second}}
	completer := retryCompleter{chat: chat}
	compiler := &theater.RoleCompiler{Store: store, Chat: completer}
	reader := &theater.BookReader{Store: store, Corpus: corpus, Chat: completer, Model: cfg.Model, Scope: role.Scope}
	architect := &theater.CharacterArchitect{Mode: theater.InitModeAgent, Scope: role.Scope, Store: store, Corpus: corpus, Reader: reader, Compiler: compiler, Chat: completer, AssessmentChat: completer, CoverageChat: completer, EvidenceQueryChat: completer, CriticChat: completer, Model: cfg.Model}
	fmt.Printf("rebuild started role=%s documents=%d chunks=%d run=%s\n", role.DisplayName, report.Documents, report.AvailableChunks, run.ID)
	run, err = architect.Continue(ctx, run.ID)
	report.Initialization = run
	report.Usage = chat.Usage()
	if err != nil {
		fail(&report, err)
	}
	report.RebuiltRole, _ = store.GetDefinition(ctx, role.ID)
	report.Claims, _ = store.ListClaims(ctx, role.ID)
	report.Readings, _ = store.ListBookReadings(ctx, role.ID)
	for _, reading := range report.Readings {
		report.ReadChunks += len(reading.ReadChunkIDs)
		fmt.Printf("read document=%s status=%s chapters=%d chunks=%d\n", reading.DocumentID, reading.Status, len(reading.Chapters), len(reading.ReadChunkIDs))
	}
	if run.BlueprintID != "" {
		report.Blueprint, _ = store.GetBlueprint(ctx, run.BlueprintID)
	}
	if run.CritiqueID != "" {
		report.Critique, _ = store.GetCritique(ctx, run.CritiqueID)
	}
	fmt.Printf("rebuild finished status=%s claims=%d readings=%d tokens=%d\n", run.Status, len(report.Claims), len(report.Readings), report.Usage.TotalTokens)
}

func samePath(left, right string) bool {
	a, _ := filepath.Abs(left)
	b, _ := filepath.Abs(right)
	return a == b
}

func writeReport(path string, report rebuildReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func fail(report *rebuildReport, err error) {
	report.Error = err.Error()
	log.Fatal(err)
}
