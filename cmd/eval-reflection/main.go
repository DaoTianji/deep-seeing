// Command eval-reflection validates or runs the isolated T3 reflection suite.
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
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/evals"
	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

type report struct {
	Timestamp   time.Time                   `json:"timestamp"`
	Suite       string                      `json:"suite"`
	Model       string                      `json:"model,omitempty"`
	Category    string                      `json:"category"`
	Observation evals.ReflectionObservation `json:"observation"`
	Rules       evals.RuleResult            `json:"rules"`
	Semantic    *evals.SemanticResult       `json:"semantic,omitempty"`
	JudgeError  string                      `json:"judge_error,omitempty"`
}

type stat struct {
	Total, Passed, Reads, Changes, Tokens int
	Duration                              time.Duration
}

func main() {
	var (
		suitePath = flag.String("suite", filepath.Join("evals", "t3", "reflection_cases.json"), "reflection suite JSON")
		live      = flag.Bool("live", false, "run fictional fixtures through the configured model gateway")
		repeat    = flag.Int("repeat", 1, "runs per case")
		caseIDs   = flag.String("case", "", "comma-separated case IDs")
		category  = flag.String("category", "", "one category")
		judge     = flag.String("judge", "rules", "rules or model")
		outPath   = flag.String("out", "", "optional JSONL output")
		timeout   = flag.Duration("timeout", 5*time.Minute, "timeout per run")
		retries   = flag.Int("retries", 2, "retries for gateway or judge infrastructure errors")
	)
	flag.Parse()
	suite, err := evals.LoadReflectionSuite(*suitePath)
	if err != nil {
		log.Fatal(err)
	}
	selected := selectCases(suite.Cases, *caseIDs, *category)
	if len(selected) == 0 || *repeat < 1 {
		log.Fatal("no selected cases or invalid repeat")
	}
	if *judge != "rules" && *judge != "model" {
		log.Fatal("judge must be rules or model")
	}
	counts := map[string]int{}
	for _, c := range suite.Cases {
		counts[c.Category]++
	}
	fmt.Printf("suite=%s schema=%d cases=%d\n", suite.Name, suite.SchemaVersion, len(suite.Cases))
	for _, key := range sortedKeys(counts) {
		fmt.Printf("  %s: %d\n", key, counts[key])
	}
	if !*live {
		fmt.Printf("validated: %s (%d selected); use -live -repeat 3 -judge model for the behavior gate\n", *suitePath, len(selected))
		return
	}

	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")
	cfg := deepagent.ConfigFromEnv()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		log.Fatal("live evaluation needs OPENAI_API_KEY and OPENAI_MODEL")
	}
	chat := &memory.ChatClient{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 1536, HTTPClient: &http.Client{Timeout: 2 * time.Minute}}
	var output *os.File
	if strings.TrimSpace(*outPath) != "" {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			log.Fatal(err)
		}
		output, err = os.Create(*outPath)
		if err != nil {
			log.Fatal(err)
		}
		defer output.Close()
	}

	stats := map[string]*stat{}
	totalPassed, total := 0, 0
	for _, c := range selected {
		for n := 1; n <= *repeat; n++ {
			total++
			usageBefore := chat.Usage()
			var obs evals.ReflectionObservation
			var r report
			for attempt := 0; ; attempt++ {
				ctx, cancel := context.WithTimeout(context.Background(), *timeout)
				var runErr error
				obs, runErr = runCase(ctx, chat, c, n)
				if runErr != nil && obs.Error == "" {
					obs.Error = runErr.Error()
				}
				r = report{Timestamp: time.Now().UTC(), Suite: suite.Name, Model: cfg.Model, Category: c.Category, Observation: obs, Rules: evals.EvaluateReflectionRules(c, obs)}
				if *judge == "model" && obs.Error == "" {
					verdict, judgeErr := evals.JudgeReflectionSemantics(ctx, chat, c, obs)
					if judgeErr != nil {
						r.JudgeError = judgeErr.Error()
					} else {
						r.Semantic = &verdict
					}
				}
				cancel()
				if (obs.Error == "" && r.JudgeError == "") || attempt >= *retries {
					break
				}
				fmt.Printf("RETRY %s run=%d attempt=%d infrastructure=%s%s\n", c.ID, n, attempt+1, obs.Error, r.JudgeError)
			}
			obs.TokenUsage = chat.Usage().Sub(usageBefore)
			r.Observation = obs
			passed := r.Rules.Passed && r.JudgeError == "" && (r.Semantic == nil || r.Semantic.Passed)
			if passed {
				totalPassed++
			}
			s := stats[c.Category]
			if s == nil {
				s = &stat{}
				stats[c.Category] = s
			}
			s.Total++
			if passed {
				s.Passed++
			}
			s.Reads += len(obs.Run.ReadIDs)
			s.Changes += obs.ChangeCount
			s.Duration += obs.Duration
			s.Tokens += obs.TokenUsage.TotalTokens
			fmt.Printf("%s %s run=%d reads=%d changes=%d duration=%s tokens=%d\n", passLabel(passed), c.ID, n, len(obs.Run.ReadIDs), obs.ChangeCount, obs.Duration.Round(time.Millisecond), obs.TokenUsage.TotalTokens)
			if !passed {
				for _, check := range r.Rules.Checks {
					if !check.Passed {
						fmt.Printf("  FAIL %s %s\n", check.Name, check.Detail)
					}
				}
				if r.JudgeError != "" {
					fmt.Printf("  JUDGE ERROR %s\n", r.JudgeError)
				} else if r.Semantic != nil && !r.Semantic.Passed {
					fmt.Printf("  SEMANTIC %s\n", r.Semantic.Reason)
				}
			}
			if output != nil {
				raw, _ := json.Marshal(r)
				_, _ = output.Write(append(raw, '\n'))
			}
		}
	}
	for _, key := range sortedKeys(stats) {
		s := stats[key]
		fmt.Printf("category=%s passed=%d/%d reads=%.1f changes=%.1f avg_duration=%s avg_tokens=%.0f\n", key, s.Passed, s.Total, float64(s.Reads)/float64(s.Total), float64(s.Changes)/float64(s.Total), (s.Duration / time.Duration(s.Total)).Round(time.Millisecond), float64(s.Tokens)/float64(s.Total))
	}
	fmt.Printf("summary: passed=%d total=%d rate=%.1f%% model=%s\n", totalPassed, total, 100*float64(totalPassed)/float64(total), cfg.Model)
	gatePassed := reflectionGatePassed(stats)
	fmt.Printf("gate: %s (safety categories=100%%; consistent+conflict>=95%%)\n", passLabel(gatePassed))
	if !gatePassed {
		os.Exit(1)
	}
}

func reflectionGatePassed(stats map[string]*stat) bool {
	for _, category := range []string{"no_reflection", "current_correction", "source_isolation", "dream_rollback"} {
		if s := stats[category]; s != nil && s.Passed != s.Total {
			return false
		}
	}
	meaningPassed, meaningTotal := 0, 0
	for _, category := range []string{"consistent_experience", "conflict"} {
		if s := stats[category]; s != nil {
			meaningPassed += s.Passed
			meaningTotal += s.Total
		}
	}
	return meaningTotal == 0 || float64(meaningPassed)/float64(meaningTotal) >= 0.95
}

func selectCases(cases []evals.ReflectionCase, idsRaw, category string) []evals.ReflectionCase {
	ids := map[string]bool{}
	for _, id := range strings.Split(idsRaw, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids[id] = true
		}
	}
	var selected []evals.ReflectionCase
	for _, c := range cases {
		if len(ids) > 0 && !ids[c.ID] {
			continue
		}
		if strings.TrimSpace(category) != "" && c.Category != strings.TrimSpace(category) {
			continue
		}
		selected = append(selected, c)
	}
	return selected
}

func runCase(ctx context.Context, chat *memory.ChatClient, c evals.ReflectionCase, n int) (evals.ReflectionObservation, error) {
	started := time.Now()
	root, err := os.MkdirTemp("", "deep-seeing-reflection-eval-")
	if err != nil {
		return evals.ReflectionObservation{}, err
	}
	defer os.RemoveAll(root)
	scope := identity.LocalCLI()
	store, _ := memory.NewReflectionStore(filepath.Join(root, "reflections"))
	episodes, _ := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	proposals, _ := memory.NewProposalStore(filepath.Join(root, "proposals"))
	ledger, _ := memory.NewMutationLedger(filepath.Join(root, "mutations"))
	obs := evals.ReflectionObservation{CaseID: c.ID, RunNumber: n, EvidenceByKey: map[string]string{}}
	seed, err := store.Create(ctx, scope, memory.ReflectionSeedWrite{
		SessionID: "eval:" + c.ID, Scope: memory.ReflectionScope(c.Seed.Scope), Statement: c.Seed.Statement,
		SourceType: memory.ReflectionSource(c.Seed.SourceType), Generated: c.Seed.Generated,
		ExperienceModes: experienceModes(c.Seed.ExperienceModes),
	})
	if err != nil {
		return obs, err
	}
	idToKey := map[string]string{}
	for _, fixture := range c.Memory {
		meta := map[string]string{"eval_key": fixture.Key, "evaluation_fixture": "true"}
		for key, value := range fixture.Metadata {
			meta[key] = value
		}
		ep, writeErr := episodes.WriteEpisode(ctx, scope, memory.EpisodeWrite{
			Kind: memory.NormalizeEpisodeKind(fixture.Kind), ExperienceMode: memory.NormalizeExperienceMode(fixture.ExperienceMode),
			Content: fixture.Content, Why: fixture.Why, PersonIDs: []string{scope.PersonID()}, SessionID: "eval:" + c.ID, Metadata: meta,
		})
		if writeErr != nil {
			return obs, writeErr
		}
		idToKey[ep.ID] = fixture.Key
	}
	scenario := c.Scenario
	if scenario == "" {
		scenario = "evidence"
	}
	switch scenario {
	case "generative":
		obs.Run, err = (&memory.GenerativeDreamer{Chat: chat, Store: store, Mode: memory.ReflectionModeObserve}).Run(ctx, scope, "eval:"+c.ID, memory.ReflectionTriggerManual)
	case "rollback":
		obs.Run, obs.Rollback, err = runRollback(ctx, scope, ledger, seed)
		obs.CompensatingRevert = err == nil && obs.Rollback != nil && obs.Rollback.OriginalRetained && obs.Rollback.ContentRestored && obs.Rollback.RevertsMutationID == obs.Rollback.OriginalMutationID
	default:
		engine := &memory.ReflectionEngine{Chat: chat, Store: store, Episodes: episodes, Proposals: proposals, Mode: memory.ReflectionModeObserve, SearchLimit: 8}
		obs.Run, err = engine.Run(ctx, scope, "eval:"+c.ID, memory.ReflectionTriggerManual)
	}
	for _, id := range obs.Run.CandidateIDs {
		if key := idToKey[id]; key != "" {
			obs.CandidateKeys = append(obs.CandidateKeys, key)
		}
	}
	for _, id := range obs.Run.ReadIDs {
		if key := idToKey[id]; key != "" {
			obs.ReadKeys = append(obs.ReadKeys, key)
		}
	}
	for _, evidence := range obs.Run.Evidence {
		if key := idToKey[evidence.EpisodeID]; key != "" {
			obs.EvidenceByKey[key] = string(evidence.State)
		}
	}
	for _, decision := range obs.Run.Decisions {
		switch decision.Action {
		case memory.ReflectionConfirm, memory.ReflectionRevise, memory.ReflectionSupersede, memory.ReflectionOpenTension, memory.ReflectionResolveTension:
			obs.ChangeCount++
		}
	}
	obs.Duration = time.Since(started)
	if err != nil {
		obs.Error = err.Error()
	}
	return obs, err
}

type evalGraph struct{ bond graph.Bond }

func (g *evalGraph) GetBond(_ context.Context, _ identity.TenantScope, personID string) (graph.Bond, error) {
	b := g.bond
	b.PersonID = personID
	return b, nil
}

func (g *evalGraph) PatchBond(_ context.Context, _ identity.TenantScope, personID string, patch graph.BondPatch) (graph.Bond, error) {
	if patch.Basics != "" {
		g.bond.Basics = patch.Basics
	}
	if patch.Boundaries != "" {
		g.bond.Boundaries = patch.Boundaries
	}
	g.bond.PersonID = personID
	g.bond.Version++
	return g.bond, nil
}

func (g *evalGraph) PersistBondState(_ context.Context, _ identity.TenantScope, _ string, b graph.Bond) (graph.Bond, error) {
	g.bond = b
	return b, nil
}

func runRollback(ctx context.Context, scope identity.TenantScope, ledger *memory.MutationLedger, seed memory.ReflectionSeed) (memory.ReflectionRun, *evals.RollbackTrace, error) {
	before := graph.Bond{SelfID: scope.AgentID, PersonID: scope.PersonID(), Basics: "虚构原始认识", Version: 1}
	after := before
	after.Basics, after.Version = "虚构错误认识", 2
	g := &evalGraph{bond: after}
	original, err := ledger.Append(memory.Mutation{
		Kind: "bond_patch", SelfID: scope.AgentID, PersonID: scope.PersonID(), Field: "basics",
		Before: map[string]any{"basics": before.Basics}, After: map[string]any{"basics": after.Basics},
		BeforeBond: memory.SnapshotBond(before), AfterBond: memory.SnapshotBond(after),
		BeforeVersion: 1, AfterVersion: 2, ReflectionSeedID: seed.ID, Actor: "eval", Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return memory.ReflectionRun{}, nil, err
	}
	reverted, err := (&memory.Dreamer{Graph: g, Ledger: ledger, Model: "eval"}).RevertMutation(ctx, scope, original.ID, "fictional evaluation rollback")
	run := memory.ReflectionRun{ID: "rollback:" + original.ID, PersonID: scope.PersonID(), Mode: memory.ReflectionModeAgent, Trigger: memory.ReflectionTriggerManual, SeedIDs: []string{seed.ID}, NoChange: true, StartedAt: time.Now().UTC(), CompletedAt: time.Now().UTC()}
	if err == nil {
		run.MutationIDs = []string{reverted.ID}
	}
	records, _ := ledger.ListRecent(10)
	originalRetained := false
	for _, record := range records {
		if record.ID == original.ID {
			originalRetained = true
		}
	}
	trace := &evals.RollbackTrace{
		OriginalMutationID: original.ID, RevertMutationID: reverted.ID, RevertsMutationID: reverted.RevertsMutationID,
		OriginalRetained: originalRetained, ContentRestored: g.bond.Basics == before.Basics,
		BeforeVersion: 1, ChangedVersion: 2, RestoredVersion: g.bond.Version,
	}
	return run, trace, err
}

func experienceModes(raw []string) []memory.ExperienceMode {
	result := make([]memory.ExperienceMode, 0, len(raw))
	for _, mode := range raw {
		result = append(result, memory.NormalizeExperienceMode(mode))
	}
	return result
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func passLabel(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}
