package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/evals"
	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/prompt"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/tools"
	"deep-seeing/internal/workspace"
)

type attentionLiveOptions struct {
	Repeat  int
	Judge   string
	OutPath string
	Timeout time.Duration
}

type attentionRunReport struct {
	Timestamp   time.Time                      `json:"timestamp"`
	Suite       string                         `json:"suite"`
	Schema      int                            `json:"schema_version"`
	CaseID      string                         `json:"case_id"`
	Category    string                         `json:"category"`
	Run         int                            `json:"run"`
	Turn        int                            `json:"turn"`
	Model       string                         `json:"model"`
	Judge       string                         `json:"judge"`
	Observation evals.AttentionTurnObservation `json:"observation"`
	Rules       evals.RuleResult               `json:"rules"`
	Semantic    *evals.SemanticResult          `json:"semantic,omitempty"`
	JudgeError  string                         `json:"judge_error,omitempty"`
}

type staticBondReader struct{ bond graph.Bond }

func (r staticBondReader) GetBond(context.Context, identity.TenantScope, string) (graph.Bond, error) {
	return r.bond, nil
}

func runAttentionLive(suite evals.AttentionSuite, selected []evals.AttentionCase, opt attentionLiveOptions) error {
	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")
	cfg := deepagent.ConfigFromEnv()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("live evaluation needs OPENAI_API_KEY and OPENAI_MODEL")
	}
	var reportFile *os.File
	var err error
	if opt.OutPath != "" {
		if err := os.MkdirAll(filepath.Dir(opt.OutPath), 0o755); err != nil {
			return err
		}
		reportFile, err = os.Create(opt.OutPath)
		if err != nil {
			return err
		}
		defer reportFile.Close()
	}
	var judgeModel *memory.ChatClient
	if opt.Judge == "model" {
		judgeModel = &memory.ChatClient{
			APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 1024,
		}
	}

	total, passed, totalTokens := 0, 0, 0
	infrastructureRetries := 0
	var totalDuration time.Duration
	for _, c := range selected {
		caseRetries := 0
		for run := 1; run <= opt.Repeat; run++ {
			ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
			observations, runErr := runAttentionCase(ctx, cfg, c, run)
			if runErr != nil {
				cancel()
				return fmt.Errorf("%s run %d setup: %w", c.ID, run, runErr)
			}
			if infrastructureErr := attentionInfrastructureError(observations); infrastructureErr != "" {
				caseRetries++
				infrastructureRetries++
				fmt.Printf("RETRY %s run=%d infrastructure=%s\n", c.ID, run, infrastructureErr)
				if reportFile != nil {
					for index, obs := range observations {
						report := attentionRunReport{
							Timestamp: time.Now().UTC(), Suite: suite.Name, Schema: suite.SchemaVersion,
							CaseID: c.ID, Category: c.Category, Run: run, Turn: index + 1,
							Model: cfg.Model, Judge: "infrastructure", Observation: obs,
							Rules: evals.EvaluateAttentionTurnRules(c.Turns[index].Expect, obs),
						}
						raw, _ := json.Marshal(report)
						if _, err := reportFile.Write(append(raw, '\n')); err != nil {
							cancel()
							return err
						}
					}
				}
				cancel()
				if caseRetries > 3 {
					return fmt.Errorf("%s exceeded 3 infrastructure retries", c.ID)
				}
				run--
				continue
			}
			for index, obs := range observations {
				total++
				report := attentionRunReport{
					Timestamp: time.Now().UTC(), Suite: suite.Name, Schema: suite.SchemaVersion,
					CaseID: c.ID, Category: c.Category, Run: run, Turn: index + 1,
					Model: cfg.Model, Judge: opt.Judge, Observation: obs,
					Rules: evals.EvaluateAttentionTurnRules(c.Turns[index].Expect, obs),
				}
				if judgeModel != nil {
					if strings.TrimSpace(obs.Answer) == "" {
						report.JudgeError = "answer empty"
					} else {
						verdict, judgeErr := evals.JudgeAttentionSemantics(ctx, judgeModel, c, index, obs)
						if judgeErr != nil {
							report.JudgeError = judgeErr.Error()
						} else {
							report.Semantic = &verdict
						}
					}
				}
				runPassed := report.Rules.Passed
				semanticLabel := "not-run"
				if judgeModel != nil {
					runPassed = report.Rules.Passed && report.JudgeError == "" && report.Semantic != nil && report.Semantic.Passed
					if report.JudgeError != "" {
						semanticLabel = "error"
					} else if report.Semantic != nil && report.Semantic.Passed {
						semanticLabel = "pass"
					} else {
						semanticLabel = "fail"
					}
				}
				if runPassed {
					passed++
				}
				totalTokens += obs.TokenUsage.TotalTokens
				totalDuration += obs.Duration
				fmt.Printf("%s %s run=%d turn=%d rules=%t semantic=%s reads=%d decisions=%d center=%d tokens=%d duration=%s\n",
					attentionPassLabel(runPassed), c.ID, run, index+1, report.Rules.Passed, semanticLabel,
					len(obs.ReadKeys), len(obs.AttentionDecisions), countAttentionTier(obs.Attention, attention.Center),
					obs.TokenUsage.TotalTokens, obs.Duration.Round(time.Millisecond))
				if !runPassed {
					for _, check := range report.Rules.Checks {
						if !check.Passed {
							fmt.Printf("  rule %s: %s\n", check.Name, check.Detail)
						}
					}
					if report.JudgeError != "" {
						fmt.Printf("  judge error: %s\n", report.JudgeError)
					} else if report.Semantic != nil && !report.Semantic.Passed {
						fmt.Printf("  semantic: %s\n", report.Semantic.Reason)
					}
				}
				if reportFile != nil {
					raw, _ := json.Marshal(report)
					if _, err := reportFile.Write(append(raw, '\n')); err != nil {
						cancel()
						return err
					}
				}
			}
			cancel()
		}
	}
	if total == 0 {
		return fmt.Errorf("no attention turns ran")
	}
	fmt.Printf("summary: passed=%d total=%d rate=%.1f%% infra_retries=%d avg_tokens=%.1f avg_duration=%s judge=%s model=%s\n",
		passed, total, 100*float64(passed)/float64(total), infrastructureRetries, float64(totalTokens)/float64(total),
		(totalDuration / time.Duration(total)).Round(time.Millisecond), opt.Judge, cfg.Model)
	if passed != total {
		return fmt.Errorf("attention acceptance failed: %d/%d turns passed", passed, total)
	}
	return nil
}

func attentionInfrastructureError(observations []evals.AttentionTurnObservation) string {
	var failures []string
	for _, obs := range observations {
		message := strings.TrimSpace(obs.InfrastructureError)
		if message == "" {
			message = strings.TrimSpace(obs.Error)
		}
		if message != "" {
			failures = append(failures, fmt.Sprintf("turn %d: %s", obs.Turn, message))
		}
	}
	return strings.Join(failures, "; ")
}

func runAttentionCase(ctx context.Context, cfg deepagent.Config, c evals.AttentionCase, run int) ([]evals.AttentionTurnObservation, error) {
	root, err := os.MkdirTemp("", "deep-seeing-attention-eval-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root)
	scope := identity.LocalCLI()
	keyByID := map[string]string{}
	idByKey := map[string]attention.Decision{}
	register := func(source contextsource.Source, id, key string) {
		fullKey := string(source) + ":" + key
		keyByID[string(source)+"\x00"+id] = fullKey
		idByKey[fullKey] = attention.Decision{Source: source, ID: id}
	}

	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		return nil, err
	}
	for _, fixture := range c.Episodes {
		item, err := episodes.WriteEpisode(ctx, scope, memory.EpisodeWrite{
			Kind: memory.NormalizeEpisodeKind(fixture.Kind), Content: fixture.Content, Why: fixture.Why,
			PersonIDs: []string{scope.PersonID()},
		})
		if err != nil {
			return nil, err
		}
		register(contextsource.Episode, item.ID, fixture.Key)
	}
	scenes, err := memory.NewSceneStore(filepath.Join(root, "scenes"))
	if err != nil {
		return nil, err
	}
	for _, fixture := range c.Scenes {
		item, err := scenes.Write(scope, memory.SceneNorm{Title: fixture.Title, Keywords: fixture.Keywords, Body: fixture.Body})
		if err != nil {
			return nil, err
		}
		register(contextsource.SceneNorm, item.ID, fixture.Key)
	}
	proposals, err := memory.NewProposalStore(filepath.Join(root, "proposals"))
	if err != nil {
		return nil, err
	}
	for _, fixture := range c.Proposals {
		item, err := proposals.Enqueue(ctx, scope, memory.ProposalWrite{
			PersonID: scope.PersonID(), Field: fixture.Field, SuggestedText: fixture.Text,
			Rationale: fixture.Rationale, Hypothesis: memory.Hypothesis(fixture.Hypothesis), Source: "synthetic_eval",
		})
		if err != nil {
			return nil, err
		}
		register(contextsource.Proposal, item.ID, fixture.Key)
	}
	workspaces, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		return nil, err
	}
	for _, fixture := range c.Workspaces {
		item, err := workspaces.Create(workspace.Write{
			Type: workspace.NormalizeType(fixture.Type), Status: workspace.NormalizeStatus(fixture.Status),
			Title: fixture.Title, Summary: fixture.Summary, Body: fixture.Body,
			Actor: "eval", RevisionNote: "synthetic fixture",
		})
		if err != nil {
			return nil, err
		}
		register(contextsource.Workspace, item.ID, fixture.Key)
	}
	intents, err := intent.OpenStore(filepath.Join(root, "runtime"))
	if err != nil {
		return nil, err
	}
	defer intents.Close()
	for index, fixture := range c.Intents {
		kind := intent.NormalizeKind(fixture.Kind)
		var interval time.Duration
		if kind == intent.IntentRecurring {
			interval = 24 * time.Hour
		}
		item, err := intents.Create(ctx, intent.CreateInput{
			AgentID: scope.AgentID, Kind: kind, Title: fixture.Title, Body: fixture.Body,
			DueAt: time.Now().UTC().Add(time.Duration(index+1) * 24 * time.Hour), Interval: interval,
		})
		if err != nil {
			return nil, err
		}
		register(contextsource.Intent, item.ID, fixture.Key)
	}

	bond := graph.Bond{SelfID: scope.AgentID, PersonID: scope.PersonID(), Version: 1}
	for index, fixture := range c.Bond {
		slot, err := graph.NormalizeSlot(fixture.Slot)
		if err != nil {
			return nil, err
		}
		bond.Items = append(bond.Items, graph.BondItem{
			ID: fmt.Sprintf("eval:%s:%d", fixture.Key, index), Slot: slot,
			Claim: fixture.Claim, Source: "explicit", Status: "active",
		})
	}
	norms := runtime.NewNormSnapshotCache(staticBondReader{bond: bond}, scope)
	focus := runtime.NewSessionTaskContextFocusStore()
	attentionStore := attention.NewSessionStore(attention.DefaultCapacity())
	sessionID := fmt.Sprintf("eval-attention-%s-%d", strings.ToLower(c.ID), run)
	if len(c.Seed) > 0 {
		decisions := make([]attention.Decision, 0, len(c.Seed))
		for _, seed := range c.Seed {
			decision, ok := idByKey[seed.Key]
			if !ok {
				return nil, fmt.Errorf("attention seed %q has no generated id", seed.Key)
			}
			decision.Target = seed.Tier
			decisions = append(decisions, decision)
		}
		if _, err := attentionStore.Apply(sessionID, decisions); err != nil {
			return nil, err
		}
	}
	toolList, err := tools.All(tools.Deps{
		Scope: scope, Episodes: episodes, Scenes: scenes, Proposals: proposals,
		Workspace: workspaces, Intents: intents, SessionID: sessionID, Model: cfg.Model,
		RecallMode: string(runtime.RecallModeAgent), TaskContextFocus: focus, Attention: attentionStore,
		Stores: map[string]string{"episode_store": "isolated", "scene_store": "isolated", "proposals": "isolated", "workspace_store": "isolated", "intent_store": "isolated", "context_graph": "isolated", "attention": "session"},
	})
	if err != nil {
		return nil, err
	}
	var service *runtime.Service
	agent, err := deepagent.New(ctx, cfg, toolList, func() string {
		if service == nil {
			return ""
		}
		return service.SystemProvider()
	})
	if err != nil {
		return nil, err
	}
	sourceStates := map[contextsource.Source]string{}
	for _, source := range []contextsource.Source{contextsource.Bond, contextsource.SceneNorm, contextsource.Workspace, contextsource.Intent, contextsource.Proposal, contextsource.Episode} {
		sourceStates[source] = "isolated"
	}
	service, err = runtime.New(runtime.Options{
		Scope: scope, SessionID: sessionID, STM: memory.NewSTM(40), RecallMode: runtime.RecallModeAgent,
		Norms: norms, TaskContext: runtime.NewStoreTaskContextProvider(workspaces, intents, focus),
		Attention: attentionStore, Agent: agent, Capability: prompt.CapabilityBlurb,
		Model: cfg.Model, ContextSources: sourceStates,
	})
	if err != nil {
		return nil, err
	}

	observations := make([]evals.AttentionTurnObservation, 0, len(c.Turns))
	for index, turn := range c.Turns {
		obs := evals.AttentionTurnObservation{Turn: index + 1}
		var reads []observe.ContextReadTrace
		var uses []observe.ContextUseTrace
		started := time.Now()
		result, turnErr := service.StreamTurnWithHooks(ctx, turn.UserText, runtime.TurnHooks{
			OnToolStart:   func(name string) { obs.ToolStarts = append(obs.ToolStarts, name) },
			OnContextRead: func(event observe.ContextReadTrace) { reads = append(reads, event) },
			OnContextUse:  func(event observe.ContextUseTrace) { uses = append(uses, event) },
			OnAttentionDecision: func(event observe.AttentionDecisionTrace) {
				obs.AttentionDecisions = append(obs.AttentionDecisions, event)
			},
		})
		obs.Answer, obs.TokenUsage, obs.Duration = result.Answer, result.TokenUsage, time.Since(started)
		if turnErr != nil {
			obs.Error = turnErr.Error()
		}
		if len(result.Errors) > 0 {
			obs.InfrastructureError = strings.Join(result.Errors, "; ")
		}
		for _, event := range reads {
			if event.OK {
				obs.ReadKeys = appendAttentionUnique(obs.ReadKeys, keyByID[string(event.Source)+"\x00"+event.ID])
			}
		}
		for _, event := range uses {
			if event.Disposition == "dismissed" {
				obs.DismissedKeys = appendAttentionUnique(obs.DismissedKeys, keyByID[string(event.Source)+"\x00"+event.ID])
			} else {
				obs.UsedKeys = appendAttentionUnique(obs.UsedKeys, keyByID[string(event.Source)+"\x00"+event.ID])
			}
		}
		obs.Attention = map[string]attention.Tier{}
		obs.AttentionIdleTurns = map[string]int{}
		for _, item := range attentionStore.Snapshot(sessionID).Items {
			key := keyByID[string(item.Source)+"\x00"+item.ID]
			if key != "" {
				obs.Attention[key] = item.Tier
				obs.AttentionIdleTurns[key] = item.IdleTurns
			}
		}
		observations = append(observations, obs)
		if turnErr != nil {
			break
		}
	}
	return observations, nil
}

func selectAttentionCases(cases []evals.AttentionCase, id string) []evals.AttentionCase {
	if id == "" {
		return cases
	}
	for _, c := range cases {
		if c.ID == id {
			return []evals.AttentionCase{c}
		}
	}
	return nil
}

func appendAttentionUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func countAttentionTier(items map[string]attention.Tier, tier attention.Tier) int {
	count := 0
	for _, value := range items {
		if value == tier {
			count++
		}
	}
	return count
}

func attentionPassLabel(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}
