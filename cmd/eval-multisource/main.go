// Command eval-multisource validates or runs the T2.4 multi-source behavior suite.
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

type runReport struct {
	Timestamp   time.Time                    `json:"timestamp"`
	Suite       string                       `json:"suite"`
	Schema      int                          `json:"schema_version"`
	Category    string                       `json:"category"`
	Model       string                       `json:"model"`
	Observation evals.MultiSourceObservation `json:"observation"`
	Rules       evals.RuleResult             `json:"rules"`
}

type staticBondReader struct{ bond graph.Bond }

func (r staticBondReader) GetBond(context.Context, identity.TenantScope, string) (graph.Bond, error) {
	return r.bond, nil
}

func main() {
	var (
		suitePath = flag.String("suite", filepath.Join("evals", "t2", "multisource_cases.json"), "multi-source suite JSON")
		live      = flag.Bool("live", false, "run the real Agent in isolated synthetic sandboxes")
		repeat    = flag.Int("repeat", 1, "runs per case")
		caseID    = flag.String("case", "", "run one case ID")
		outPath   = flag.String("out", "", "optional ignored JSONL report path")
		timeout   = flag.Duration("timeout", 5*time.Minute, "timeout per live case")
	)
	flag.Parse()

	suite, err := evals.LoadMultiSourceSuite(*suitePath)
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
	fmt.Printf("suite=%s schema=%d cases=%d selected=%d\n", suite.Name, suite.SchemaVersion, len(suite.Cases), len(selected))
	if !*live {
		fmt.Printf("validated: %s; live mode sends only these synthetic fixtures to the configured model gateway\n", *suitePath)
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

	total, passed, totalTokens := 0, 0, 0
	for _, c := range selected {
		for run := 1; run <= *repeat; run++ {
			total++
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			obs, runErr := runCase(ctx, cfg, c, run)
			cancel()
			if runErr != nil {
				log.Fatalf("%s run %d setup: %v", c.ID, run, runErr)
			}
			rules := evals.EvaluateMultiSourceRules(c, obs)
			if rules.Passed {
				passed++
			}
			totalTokens += obs.TokenUsage.TotalTokens
			fmt.Printf("%s %s run=%d candidates=%d reads=%d uses=%d tokens=%d duration=%s\n",
				passLabel(rules.Passed), c.ID, run, len(obs.CandidateKeys), len(obs.ReadKeys), len(obs.UsedKeys)+len(obs.DismissedKeys),
				obs.TokenUsage.TotalTokens, obs.Duration.Round(time.Millisecond))
			if !rules.Passed {
				for _, check := range rules.Checks {
					if !check.Passed {
						fmt.Printf("  rule %s: %s\n", check.Name, check.Detail)
					}
				}
			}
			if reportFile != nil {
				report := runReport{Timestamp: time.Now().UTC(), Suite: suite.Name, Schema: suite.SchemaVersion,
					Category: c.Category, Model: cfg.Model, Observation: obs, Rules: rules}
				raw, _ := json.Marshal(report)
				_, _ = reportFile.Write(append(raw, '\n'))
			}
		}
	}
	fmt.Printf("summary: passed=%d total=%d rate=%.1f%% avg_tokens=%.1f model=%s\n",
		passed, total, 100*float64(passed)/float64(total), float64(totalTokens)/float64(total), cfg.Model)
	if passed != total {
		os.Exit(1)
	}
}

func runCase(ctx context.Context, cfg deepagent.Config, c evals.MultiSourceCase, run int) (evals.MultiSourceObservation, error) {
	root, err := os.MkdirTemp("", "deep-seeing-multisource-eval-")
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}
	defer os.RemoveAll(root)
	scope := identity.LocalCLI()
	keyByID := map[string]string{}
	register := func(source contextsource.Source, id, key string) {
		keyByID[string(source)+"\x00"+id] = string(source) + ":" + key
	}

	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}
	for _, fixture := range c.Episodes {
		item, err := episodes.WriteEpisode(ctx, scope, memory.EpisodeWrite{
			Kind: memory.NormalizeEpisodeKind(fixture.Kind), Content: fixture.Content, Why: fixture.Why,
			PersonIDs: []string{scope.PersonID()},
		})
		if err != nil {
			return evals.MultiSourceObservation{}, err
		}
		register(contextsource.Episode, item.ID, fixture.Key)
	}
	scenes, err := memory.NewSceneStore(filepath.Join(root, "scenes"))
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}
	for _, fixture := range c.Scenes {
		item, err := scenes.Write(scope, memory.SceneNorm{Title: fixture.Title, Keywords: fixture.Keywords, Body: fixture.Body})
		if err != nil {
			return evals.MultiSourceObservation{}, err
		}
		register(contextsource.SceneNorm, item.ID, fixture.Key)
	}
	proposals, err := memory.NewProposalStore(filepath.Join(root, "proposals"))
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}
	for _, fixture := range c.Proposals {
		item, err := proposals.Enqueue(ctx, scope, memory.ProposalWrite{
			PersonID: scope.PersonID(), Field: fixture.Field, SuggestedText: fixture.Text,
			Rationale: fixture.Rationale, Hypothesis: memory.Hypothesis(fixture.Hypothesis), Source: "synthetic_eval",
		})
		if err != nil {
			return evals.MultiSourceObservation{}, err
		}
		register(contextsource.Proposal, item.ID, fixture.Key)
	}
	workspaces, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}
	for _, fixture := range c.Workspaces {
		item, err := workspaces.Create(workspace.Write{
			Type: workspace.NormalizeType(fixture.Type), Status: workspace.NormalizeStatus(fixture.Status),
			Title: fixture.Title, Summary: fixture.Summary, Body: fixture.Body,
			Actor: "eval", RevisionNote: "synthetic fixture",
		})
		if err != nil {
			return evals.MultiSourceObservation{}, err
		}
		register(contextsource.Workspace, item.ID, fixture.Key)
	}
	intents, err := intent.OpenStore(filepath.Join(root, "runtime"))
	if err != nil {
		return evals.MultiSourceObservation{}, err
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
			return evals.MultiSourceObservation{}, err
		}
		register(contextsource.Intent, item.ID, fixture.Key)
	}

	bond := graph.Bond{SelfID: scope.AgentID, PersonID: scope.PersonID(), Version: 1}
	for index, fixture := range c.Bond {
		slot, err := graph.NormalizeSlot(fixture.Slot)
		if err != nil {
			return evals.MultiSourceObservation{}, err
		}
		bond.Items = append(bond.Items, graph.BondItem{
			ID: fmt.Sprintf("eval:%s:%d", fixture.Key, index), Slot: slot,
			Claim: fixture.Claim, Source: "explicit", Status: "active",
		})
	}
	norms := runtime.NewNormSnapshotCache(staticBondReader{bond: bond}, scope)
	focus := runtime.NewSessionTaskContextFocusStore()
	sessionID := fmt.Sprintf("eval-multisource-%s-%d", strings.ToLower(c.ID), run)
	toolList, err := tools.All(tools.Deps{
		Scope: scope, Episodes: episodes, Scenes: scenes, Proposals: proposals,
		Workspace: workspaces, Intents: intents, SessionID: sessionID, Model: cfg.Model,
		RecallMode: string(runtime.RecallModeAgent), TaskContextFocus: focus,
		Stores: map[string]string{"episode_store": "isolated", "scene_store": "isolated", "proposals": "isolated", "workspace_store": "isolated", "intent_store": "isolated", "context_graph": "isolated"},
	})
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}
	var service *runtime.Service
	agent, err := deepagent.New(ctx, cfg, toolList, func() string {
		if service == nil {
			return ""
		}
		return service.SystemProvider()
	})
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}
	sourceStates := map[contextsource.Source]string{}
	for _, source := range []contextsource.Source{contextsource.Bond, contextsource.SceneNorm, contextsource.Workspace, contextsource.Intent, contextsource.Proposal, contextsource.Episode} {
		sourceStates[source] = "isolated"
	}
	service, err = runtime.New(runtime.Options{
		Scope: scope, SessionID: sessionID, STM: memory.NewSTM(40), RecallMode: runtime.RecallModeAgent,
		Norms: norms, TaskContext: runtime.NewStoreTaskContextProvider(workspaces, intents, focus),
		Agent: agent, Capability: prompt.CapabilityBlurb, Model: cfg.Model, ContextSources: sourceStates,
	})
	if err != nil {
		return evals.MultiSourceObservation{}, err
	}

	obs := evals.MultiSourceObservation{CaseID: c.ID, Run: run}
	started := time.Now()
	result, turnErr := service.StreamTurnWithHooks(ctx, c.UserText, runtime.TurnHooks{
		OnContextSource:    func(event observe.ContextSourceTrace) { obs.Sources = append(obs.Sources, event) },
		OnContextCandidate: func(event observe.ContextCandidateTrace) { obs.Candidates = append(obs.Candidates, event) },
		OnContextRead:      func(event observe.ContextReadTrace) { obs.Reads = append(obs.Reads, event) },
		OnContextUse:       func(event observe.ContextUseTrace) { obs.Uses = append(obs.Uses, event) },
	})
	obs.Answer, obs.TokenUsage, obs.Duration = result.Answer, result.TokenUsage, time.Since(started)
	if turnErr != nil {
		obs.Error = turnErr.Error()
	}
	for _, event := range obs.Candidates {
		for _, id := range event.ResultIDs {
			obs.CandidateKeys = appendUnique(obs.CandidateKeys, keyByID[string(event.Source)+"\x00"+id])
		}
	}
	for _, event := range obs.Reads {
		if event.OK {
			obs.ReadKeys = appendUnique(obs.ReadKeys, keyByID[string(event.Source)+"\x00"+event.ID])
		}
	}
	for _, event := range obs.Uses {
		key := keyByID[string(event.Source)+"\x00"+event.ID]
		if event.Disposition == "dismissed" {
			obs.DismissedKeys = appendUnique(obs.DismissedKeys, key)
		} else {
			obs.UsedKeys = appendUnique(obs.UsedKeys, key)
		}
	}
	return obs, nil
}

func selectCases(cases []evals.MultiSourceCase, id string) []evals.MultiSourceCase {
	if id == "" {
		return cases
	}
	for _, c := range cases {
		if c.ID == id {
			return []evals.MultiSourceCase{c}
		}
	}
	return nil
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
