// Command eval-recall validates or runs the versioned T2 recall behavior suite.
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
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/prompt"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/tools"
)

type runReport struct {
	Timestamp   time.Time               `json:"timestamp"`
	Suite       string                  `json:"suite"`
	Schema      int                     `json:"schema_version"`
	Category    string                  `json:"category"`
	Model       string                  `json:"model"`
	Judge       string                  `json:"judge"`
	Observation evals.RecallObservation `json:"observation"`
	Rules       evals.RuleResult        `json:"rules"`
	Semantic    *evals.SemanticResult   `json:"semantic,omitempty"`
	JudgeError  string                  `json:"judge_error,omitempty"`
}

func main() {
	var (
		suitePath = flag.String("suite", filepath.Join("evals", "t2", "recall_cases.json"), "recall suite JSON")
		live      = flag.Bool("live", false, "run the real Agent in isolated memory sandboxes")
		repeat    = flag.Int("repeat", 1, "runs per selected case")
		caseID    = flag.String("case", "", "run one case ID")
		category  = flag.String("category", "", "run one category")
		judge     = flag.String("judge", "rules", "rules or model")
		outPath   = flag.String("out", "", "optional JSONL report path")
		timeout   = flag.Duration("timeout", 5*time.Minute, "timeout per live run")
	)
	flag.Parse()

	suite, err := evals.LoadRecallSuite(*suitePath)
	if err != nil {
		log.Fatal(err)
	}
	selected := selectCases(suite.Cases, strings.TrimSpace(*caseID), strings.TrimSpace(*category))
	if len(selected) == 0 {
		log.Fatal("no cases selected")
	}
	if *repeat < 1 {
		log.Fatal("repeat must be at least 1")
	}
	if *judge != "rules" && *judge != "model" {
		log.Fatal("judge must be rules or model")
	}

	counts := suite.CategoryCounts()
	fmt.Printf("suite=%s schema=%d cases=%d\n", suite.Name, suite.SchemaVersion, len(suite.Cases))
	for _, key := range evals.SortedKeys(counts) {
		fmt.Printf("  %s: %d\n", key, counts[key])
	}
	if !*live {
		fmt.Printf("validated: %s (%d selected); use -live to run the real Agent\n", *suitePath, len(selected))
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

	var judgeModel *memory.ChatClient
	if *judge == "model" {
		judgeModel = &memory.ChatClient{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 256}
	}
	total, passed := 0, 0
	for _, c := range selected {
		for run := 1; run <= *repeat; run++ {
			total++
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			obs, setupErr := runCase(ctx, cfg, c, run)
			if setupErr != nil {
				cancel()
				log.Fatalf("%s run %d setup: %v", c.ID, run, setupErr)
			}
			report := runReport{
				Timestamp: time.Now().UTC(), Suite: suite.Name, Schema: suite.SchemaVersion,
				Category: c.Category, Model: cfg.Model, Judge: *judge,
				Observation: obs, Rules: evals.EvaluateRecallRules(c, obs),
			}
			if judgeModel != nil && strings.TrimSpace(obs.Answer) != "" {
				verdict, judgeErr := evals.JudgeRecallSemantics(ctx, judgeModel, c, obs)
				if judgeErr != nil {
					report.JudgeError = judgeErr.Error()
				} else {
					report.Semantic = &verdict
				}
			}
			cancel()
			runPassed := report.Rules.Passed && report.JudgeError == "" && (report.Semantic == nil || report.Semantic.Passed)
			if runPassed {
				passed++
			}
			semanticLabel := "not-run"
			if report.JudgeError != "" {
				semanticLabel = "error"
			} else if report.Semantic != nil && report.Semantic.Passed {
				semanticLabel = "pass"
			} else if report.Semantic != nil {
				semanticLabel = "fail"
			}
			fmt.Printf("%s %s run=%d rules=%t semantic=%s searches=%d candidates=%d duration=%s\n",
				passLabel(runPassed), c.ID, run, report.Rules.Passed, semanticLabel,
				len(obs.Searches), len(obs.CandidateIDs), obs.Duration.Round(time.Millisecond))
			if !runPassed {
				printFailures(report)
			}
			if reportFile != nil {
				raw, _ := json.Marshal(report)
				_, _ = reportFile.Write(append(raw, '\n'))
			}
		}
	}
	fmt.Printf("summary: passed=%d total=%d rate=%.1f%% judge=%s model=%s\n", passed, total, 100*float64(passed)/float64(total), *judge, cfg.Model)
	if passed != total {
		os.Exit(1)
	}
}

func runCase(ctx context.Context, cfg deepagent.Config, c evals.RecallCase, run int) (evals.RecallObservation, error) {
	root, err := os.MkdirTemp("", "deep-seeing-recall-eval-")
	if err != nil {
		return evals.RecallObservation{}, err
	}
	defer os.RemoveAll(root)
	scope := identity.LocalCLI()
	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		return evals.RecallObservation{}, err
	}
	idToKey := map[string]string{}
	for _, fixture := range c.Memory {
		meta := map[string]string{"eval_key": fixture.Key}
		for key, value := range fixture.Meta {
			meta[key] = value
		}
		ep, err := episodes.WriteEpisode(ctx, scope, memory.EpisodeWrite{
			Kind: memory.NormalizeEpisodeKind(fixture.Kind), Content: fixture.Content,
			Why: fixture.Why, PersonIDs: []string{scope.PersonID()},
			SessionID: "eval:" + c.ID, Metadata: meta,
		})
		if err != nil {
			return evals.RecallObservation{}, err
		}
		idToKey[ep.ID] = fixture.Key
	}
	sessionID := fmt.Sprintf("eval-%s-%d", strings.ToLower(c.ID), run)
	toolList, err := tools.All(tools.Deps{
		Scope: scope, Episodes: episodes, SessionID: sessionID, Model: cfg.Model,
		Stores:     map[string]string{"stm": "memory", "episode_store": "isolated", "context_graph": "unavailable"},
		RecallMode: string(runtime.RecallModeAgent),
	})
	if err != nil {
		return evals.RecallObservation{}, err
	}
	var service *runtime.Service
	reactAgent, err := deepagent.New(ctx, cfg, toolList, func() string {
		if service == nil {
			return ""
		}
		return service.SystemProvider()
	})
	if err != nil {
		return evals.RecallObservation{}, err
	}
	service, err = runtime.New(runtime.Options{
		Scope: scope, SessionID: sessionID, STM: memory.NewSTM(40), RecallMode: runtime.RecallModeAgent,
		Norms: runtime.NewNormSnapshotCache(nil, scope), Agent: reactAgent,
		Capability: prompt.CapabilityBlurb, Model: cfg.Model,
	})
	if err != nil {
		return evals.RecallObservation{}, err
	}
	var searches []observe.RecallSearchTrace
	var toolStarts []string
	started := time.Now()
	result, turnErr := service.StreamTurnWithHooks(ctx, c.UserText, runtime.TurnHooks{
		OnToolStart: func(name string) { toolStarts = append(toolStarts, name) },
		OnRecallSearch: func(search observe.RecallSearchTrace) {
			searches = append(searches, search)
		},
	})
	obs := evals.RecallObservation{
		CaseID: c.ID, Run: run, RecallMode: string(runtime.RecallModeAgent), Searches: searches,
		ToolStarts: toolStarts, Answer: result.Answer, Duration: time.Since(started),
	}
	if turnErr != nil {
		obs.Error = turnErr.Error()
	}
	obs.CandidateIDs = evals.CandidateIDs(searches)
	for _, id := range obs.CandidateIDs {
		if key := idToKey[id]; key != "" {
			obs.CandidateKeys = append(obs.CandidateKeys, key)
		}
	}
	return obs, nil
}

func selectCases(cases []evals.RecallCase, caseID, category string) []evals.RecallCase {
	var out []evals.RecallCase
	for _, c := range cases {
		if caseID != "" && c.ID != caseID {
			continue
		}
		if category != "" && c.Category != category {
			continue
		}
		out = append(out, c)
	}
	return out
}

func passLabel(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func printFailures(report runReport) {
	for _, check := range report.Rules.Checks {
		if !check.Passed {
			fmt.Printf("  rule %s: %s\n", check.Name, check.Detail)
		}
	}
	if report.JudgeError != "" {
		fmt.Printf("  judge error: %s\n", report.JudgeError)
	}
	if report.Semantic != nil && !report.Semantic.Passed {
		fmt.Printf("  semantic: %s\n", report.Semantic.Reason)
	}
}
