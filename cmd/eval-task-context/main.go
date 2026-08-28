// Command eval-task-context validates or runs the T2 task-context behavior suite.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/evals"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/prompt"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/tools"
	"deep-seeing/internal/workspace"
)

type runReport struct {
	Timestamp   time.Time                    `json:"timestamp"`
	Suite       string                       `json:"suite"`
	Schema      int                          `json:"schema_version"`
	Judge       string                       `json:"judge,omitempty"`
	Category    string                       `json:"category"`
	Model       string                       `json:"model"`
	Observation evals.TaskContextObservation `json:"observation"`
	Rules       evals.RuleResult             `json:"rules"`
}

func main() {
	var (
		suitePath = flag.String("suite", filepath.Join("evals", "t2", "task_context_cases.json"), "task context suite JSON")
		live      = flag.Bool("live", false, "run the real Agent in isolated synthetic sandboxes")
		repeat    = flag.Int("repeat", 1, "runs per case")
		caseID    = flag.String("case", "", "run one case ID or a comma-separated list")
		outPath   = flag.String("out", "", "optional ignored JSONL report path")
		timeout   = flag.Duration("timeout", 5*time.Minute, "timeout per live case")
	)
	flag.Parse()

	suite, err := evals.LoadTaskContextSuite(*suitePath)
	if err != nil {
		log.Fatal(err)
	}
	selected := selectCases(suite.Cases, strings.TrimSpace(*caseID))
	if len(selected) == 0 {
		log.Fatal("no cases selected")
	}
	if *repeat < 1 {
		log.Fatal("repeat must be at least 1")
	}
	totalTurns := 0
	for _, c := range selected {
		totalTurns += len(c.Turns)
	}
	fmt.Printf("suite=%s schema=%d cases=%d selected=%d turns=%d\n", suite.Name, suite.SchemaVersion, len(suite.Cases), len(selected), totalTurns)
	if !*live {
		fmt.Printf("validated: %s; use -live to run the real Agent\n", *suitePath)
		return
	}

	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")
	cfg := deepagent.ConfigFromEnv()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		log.Fatal("live evaluation needs OPENAI_API_KEY and OPENAI_MODEL")
	}
	var reportFile *os.File
	if strings.TrimSpace(*outPath) != "" {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			log.Fatal(err)
		}
		reportFile, err = os.Create(*outPath)
		if err != nil {
			log.Fatal(err)
		}
		defer reportFile.Close()
	}

	total, passed, tokens, turns, infrastructureRetries := 0, 0, 0, 0, 0
	for _, c := range selected {
		caseRetries := 0
		for run := 1; run <= *repeat; run++ {
			total++
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			obs, setupErr := runCase(ctx, cfg, c, run)
			cancel()
			if setupErr != nil {
				log.Fatalf("%s run %d setup: %v", c.ID, run, setupErr)
			}
			if evals.IsInfrastructureFailure(obs.Error) {
				caseRetries++
				infrastructureRetries++
				total--
				rules := evals.EvaluateTaskContextRules(c, obs)
				fmt.Printf("RETRY %s run=%d infrastructure=%s\n", c.ID, run, obs.Error)
				if reportFile != nil {
					report := runReport{
						Timestamp: time.Now().UTC(), Suite: suite.Name, Schema: suite.SchemaVersion,
						Category: c.Category, Model: cfg.Model, Judge: "infrastructure", Observation: obs, Rules: rules,
					}
					raw, _ := json.Marshal(report)
					_, _ = reportFile.Write(append(raw, '\n'))
				}
				if caseRetries > 3 {
					log.Fatalf("%s exceeded 3 infrastructure retries", c.ID)
				}
				run--
				continue
			}
			rules := evals.EvaluateTaskContextRules(c, obs)
			if rules.Passed {
				passed++
			}
			tokens += obs.TokenUsage.TotalTokens
			turns += len(obs.Turns)
			expands, focuses, searches := observationCounts(obs)
			fmt.Printf("%s %s run=%d turns=%d expansions=%d focuses=%d searches=%d tokens=%d duration=%s\n",
				passLabel(rules.Passed), c.ID, run, len(obs.Turns), expands, focuses, searches,
				obs.TokenUsage.TotalTokens, obs.Duration.Round(time.Millisecond))
			if !rules.Passed {
				for _, check := range rules.Checks {
					if !check.Passed {
						fmt.Printf("  rule %s: %s\n", check.Name, check.Detail)
					}
				}
			}
			if reportFile != nil {
				report := runReport{
					Timestamp: time.Now().UTC(), Suite: suite.Name, Schema: suite.SchemaVersion,
					Category: c.Category, Model: cfg.Model, Observation: obs, Rules: rules,
				}
				raw, _ := json.Marshal(report)
				_, _ = reportFile.Write(append(raw, '\n'))
			}
		}
	}
	fmt.Printf("summary: passed=%d total=%d rate=%.1f%% infra_retries=%d turns=%d avg_tokens=%.1f model=%s\n",
		passed, total, 100*float64(passed)/float64(total), infrastructureRetries, turns, float64(tokens)/float64(total), cfg.Model)
	if passed != total {
		os.Exit(1)
	}
}

func runCase(ctx context.Context, cfg deepagent.Config, c evals.TaskContextCase, run int) (evals.TaskContextObservation, error) {
	root, err := os.MkdirTemp("", "deep-seeing-task-context-eval-")
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	defer os.RemoveAll(root)

	wsStore, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	workspaceIDToKey := map[string]string{}
	for _, fixture := range c.Workspaces {
		doc, err := wsStore.Create(workspace.Write{
			Type: workspace.NormalizeType(fixture.Type), Status: workspace.NormalizeStatus(fixture.Status),
			Title: fixture.Title, Summary: fixture.Summary, Body: fixture.Body,
			Actor: "eval", RevisionNote: "synthetic fixture",
		})
		if err != nil {
			return evals.TaskContextObservation{}, err
		}
		workspaceIDToKey[doc.ID] = fixture.Key
		time.Sleep(time.Millisecond)
	}

	intentStore, err := intent.OpenStore(filepath.Join(root, "runtime"))
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	defer intentStore.Close()
	scope := identity.LocalCLI()
	intentIDToKey := map[string]string{}
	for index, fixture := range c.Intents {
		kind := intent.NormalizeKind(fixture.Kind)
		interval := time.Duration(0)
		if kind == intent.IntentRecurring {
			interval = 24 * time.Hour
		}
		item, err := intentStore.Create(ctx, intent.CreateInput{
			AgentID: scope.AgentID, Kind: kind, Title: fixture.Title, Body: fixture.Body,
			DueAt: time.Now().UTC().Add(time.Duration(index+1) * time.Hour), Interval: interval,
		})
		if err != nil {
			return evals.TaskContextObservation{}, err
		}
		intentIDToKey[item.ID] = fixture.Key
	}

	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	persistentBefore, err := capturePersistentState(ctx, scope, wsStore, intentStore, episodes)
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	sessionID := fmt.Sprintf("eval-context-%s-%d", strings.ToLower(c.ID), run)
	focusStore := runtime.NewSessionTaskContextFocusStore()
	toolList, err := tools.All(tools.Deps{
		Scope: scope, Episodes: episodes, Workspace: wsStore, Intents: intentStore,
		SessionID: sessionID, Model: cfg.Model, RecallMode: string(runtime.RecallModeAgent),
		TaskContextFocus: focusStore,
		Stores: map[string]string{
			"stm": "memory", "episode_store": "isolated",
			"workspace_store": "isolated", "intent_store": "isolated", "context_graph": "unavailable",
		},
	})
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	var service *runtime.Service
	reactAgent, err := deepagent.New(ctx, cfg, toolList, func() string {
		if service == nil {
			return ""
		}
		return service.SystemProvider()
	})
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	service, err = runtime.New(runtime.Options{
		Scope: scope, SessionID: sessionID, STM: memory.NewSTM(40),
		RecallMode: runtime.RecallModeAgent, Norms: runtime.NewNormSnapshotCache(nil, scope),
		TaskContext: runtime.NewStoreTaskContextProvider(wsStore, intentStore, focusStore),
		Agent:       reactAgent, Capability: prompt.CapabilityBlurb, Model: cfg.Model,
	})
	if err != nil {
		return evals.TaskContextObservation{}, err
	}

	obs := evals.TaskContextObservation{CaseID: c.ID, Run: run}
	started := time.Now()
	for index, turn := range c.Turns {
		turnObs := runTurn(ctx, service, turn.UserText, index+1, workspaceIDToKey, intentIDToKey)
		obs.Turns = append(obs.Turns, turnObs)
		addTokenUsage(&obs.TokenUsage, turnObs.TokenUsage)
		if turnObs.Error != "" {
			obs.Error = fmt.Sprintf("turn %d: %s", index+1, turnObs.Error)
			break
		}
	}
	obs.Duration = time.Since(started)
	persistentAfter, err := capturePersistentState(ctx, scope, wsStore, intentStore, episodes)
	if err != nil {
		return evals.TaskContextObservation{}, err
	}
	for _, storeName := range []string{"episode", "workspace", "intent"} {
		if persistentBefore[storeName] != persistentAfter[storeName] {
			obs.PersistentWrites = append(obs.PersistentWrites, storeName)
		}
	}
	return obs, nil
}

func capturePersistentState(ctx context.Context, scope identity.TenantScope, workspaces *workspace.Store, intents *intent.Store, episodes *memory.EpisodeStore) (map[string]string, error) {
	workspaceItems, err := workspaces.List(workspace.ListFilter{Limit: 1000})
	if err != nil {
		return nil, err
	}
	intentItems, err := intents.ListRecent(ctx, scope.AgentID, 1000)
	if err != nil {
		return nil, err
	}
	episodeItems, err := episodes.ListEpisodes(ctx, scope, 1000, true)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for name, value := range map[string]any{
		"workspace": workspaceItems,
		"intent":    intentItems,
		"episode":   episodeItems,
	} {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		out[name] = string(raw)
	}
	return out, nil
}

func runTurn(ctx context.Context, service *runtime.Service, userText string, turn int, workspaceIDToKey, intentIDToKey map[string]string) evals.TaskContextTurnObservation {
	var contextTrace *observe.TaskContextTrace
	var expansions []observe.TaskContextExpansionTrace
	var focus *observe.TaskContextFocusTrace
	var searches []observe.RecallSearchTrace
	started := time.Now()
	result, turnErr := service.StreamTurnWithHooks(ctx, userText, runtime.TurnHooks{
		OnTaskContext: func(event observe.TaskContextTrace) {
			copy := event
			contextTrace = &copy
		},
		OnContextExpand: func(event observe.TaskContextExpansionTrace) {
			expansions = append(expansions, event)
		},
		OnContextFocus: func(event observe.TaskContextFocusTrace) {
			copy := event
			focus = &copy
		},
		OnRecallSearch: func(event observe.RecallSearchTrace) {
			searches = append(searches, event)
		},
	})
	obs := evals.TaskContextTurnObservation{
		Turn: turn, Context: contextTrace, Expansions: expansions, Focus: focus, Searches: searches,
		Answer: result.Answer, Duration: time.Since(started), TokenUsage: result.TokenUsage,
	}
	if turnErr != nil {
		obs.Error = turnErr.Error()
	}
	mapTaskContextKeys(&obs, workspaceIDToKey, intentIDToKey)
	return obs
}

func mapTaskContextKeys(obs *evals.TaskContextTurnObservation, workspaceIDToKey, intentIDToKey map[string]string) {
	if obs.Context != nil {
		obs.SnapshotFocusWorkspaceKey = workspaceIDToKey[obs.Context.FocusWorkspaceID]
		obs.SnapshotFocusIntentKey = intentIDToKey[obs.Context.FocusIntentID]
	}
	if obs.Focus != nil {
		obs.FocusWorkspaceKey = workspaceIDToKey[obs.Focus.WorkspaceID]
		obs.FocusIntentKey = intentIDToKey[obs.Focus.IntentID]
	}
	for _, event := range obs.Expansions {
		if event.Error != "" {
			continue
		}
		switch event.Operation {
		case "list":
			for _, id := range event.ResultIDs {
				if event.Source == "workspace" {
					obs.ListedWorkspaceKeys = appendUnique(obs.ListedWorkspaceKeys, workspaceIDToKey[id])
				} else if event.Source == "intent" {
					obs.ListedIntentKeys = appendUnique(obs.ListedIntentKeys, intentIDToKey[id])
				}
			}
		case "read", "":
			if event.Source == "workspace" {
				obs.WorkspaceKeys = appendUnique(obs.WorkspaceKeys, workspaceIDToKey[event.ID])
			} else if event.Source == "intent" {
				obs.IntentKeys = appendUnique(obs.IntentKeys, intentIDToKey[event.ID])
			}
		}
	}
}

func observationCounts(obs evals.TaskContextObservation) (expansions, focuses, searches int) {
	for _, turn := range obs.Turns {
		expansions += len(turn.Expansions)
		searches += len(turn.Searches)
		if turn.Focus != nil {
			focuses++
		}
	}
	return expansions, focuses, searches
}

func addTokenUsage(total *observe.TokenUsageTrace, add observe.TokenUsageTrace) {
	total.PromptTokens += add.PromptTokens
	total.CompletionTokens += add.CompletionTokens
	total.ReasoningTokens += add.ReasoningTokens
	total.TotalTokens += add.TotalTokens
}

func selectCases(cases []evals.TaskContextCase, caseID string) []evals.TaskContextCase {
	var out []evals.TaskContextCase
	wanted := evals.ParseCaseIDs(caseID)
	for _, c := range cases {
		if len(wanted) == 0 || wanted[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

func appendUnique(values []string, value string) []string {
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

func passLabel(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}
