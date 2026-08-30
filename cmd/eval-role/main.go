// Command eval-role validates or runs the isolated T4 role theater suite.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
	"deep-seeing/internal/theater"
)

type report struct {
	Timestamp   time.Time             `json:"timestamp"`
	Model       string                `json:"model"`
	Category    string                `json:"category"`
	Observation evals.RoleObservation `json:"observation"`
	Rules       evals.RuleResult      `json:"rules"`
	Semantic    *evals.SemanticResult `json:"semantic,omitempty"`
}

type scriptedCompleter struct{ out string }

func (s scriptedCompleter) Complete(context.Context, string, string) (string, error) {
	return s.out, nil
}

func main() {
	var (
		suitePath = flag.String("suite", filepath.Join("evals", "t4", "role_cases.json"), "role suite JSON")
		live      = flag.Bool("live", false, "run model behavior")
		repeat    = flag.Int("repeat", 1, "runs per case")
		judge     = flag.String("judge", "model", "model or none")
		outPath   = flag.String("out", "", "JSONL output path")
		caseIDs   = flag.String("cases", "", "comma-separated case IDs")
	)
	flag.Parse()
	suite, err := evals.LoadRoleSuite(*suitePath)
	if err != nil {
		log.Fatal(err)
	}
	selected := selectCases(suite.Cases, *caseIDs)
	fmt.Printf("T4 role suite: %d/%d cases\n", len(selected), len(suite.Cases))
	if !*live {
		fmt.Println("PASS schema and fixed inventory")
		return
	}
	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")
	cfg := deepagent.ConfigFromEnv()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		log.Fatal("live evaluation needs OPENAI_API_KEY and OPENAI_MODEL")
	}
	if *repeat <= 0 {
		*repeat = 1
	}
	chat := &memory.ChatClient{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 900}
	var writer *bufio.Writer
	var file *os.File
	if *outPath != "" {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			log.Fatal(err)
		}
		file, err = os.Create(*outPath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		writer = bufio.NewWriter(file)
		defer writer.Flush()
	}
	ctx := context.Background()
	passed, total := 0, 0
	for run := 1; run <= *repeat; run++ {
		for _, c := range selected {
			obs := runCase(ctx, chat, c, run)
			rules := evals.EvaluateRoleRules(c, obs)
			item := report{Timestamp: time.Now().UTC(), Model: cfg.Model, Category: c.Category, Observation: obs, Rules: rules}
			semanticPassed := true
			if *judge == "model" && obs.Error == "" {
				verdict, judgeErr := evals.JudgeRoleSemantics(ctx, chat, c, obs)
				if judgeErr != nil {
					semanticPassed = false
					obs.Error = judgeErr.Error()
					item.Observation = obs
				} else {
					item.Semantic = &verdict
					semanticPassed = verdict.Passed
				}
			}
			total++
			ok := rules.Passed && semanticPassed
			if ok {
				passed++
			}
			fmt.Printf("%s run=%d rules=%v semantic=%v action=%s\n", c.ID, run, rules.Passed, semanticPassed, obs.DirectorAction)
			if writer != nil {
				raw, _ := json.Marshal(item)
				_, _ = writer.Write(append(raw, '\n'))
				_ = writer.Flush()
			}
		}
	}
	fmt.Printf("T4 role result: %d/%d passed\n", passed, total)
	if passed != total {
		os.Exit(1)
	}
}

func runCase(ctx context.Context, chat *memory.ChatClient, c evals.RoleCase, run int) evals.RoleObservation {
	start := time.Now()
	obs := evals.RoleObservation{CaseID: c.ID, RunNumber: run, DirectorAction: "no_change", StructuralPassed: true}
	root, err := os.MkdirTemp("", "deep-seeing-role-eval-")
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	defer os.RemoveAll(root)
	store, err := theater.NewStore(filepath.Join(root, "roles"))
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	episodes, err := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	scope := identity.TenantScope{UserID: "eval-user", AgentID: "an"}
	d, err := store.CreateDefinition(ctx, scope, theater.RoleDefinitionWrite{
		DisplayName: c.Role.DisplayName, Kind: theater.RoleKind(c.Role.Kind),
		SubjectClass: theater.SubjectClass(c.Role.SubjectClass), Identity: c.Role.Identity,
		Voice: c.Role.Voice, KnowledgeCutoff: c.Role.KnowledgeCutoff,
	})
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	var sourceID string
	if c.Role.Source != "" {
		source, addErr := store.AddSource(ctx, d.ID, "评估资料", "fixture", "", "text/plain", []byte(c.Role.Source))
		if addErr != nil {
			obs.Error = addErr.Error()
			return obs
		}
		sourceID = source.ID
		_, _ = store.AddClaim(ctx, theater.RoleClaim{
			RoleID: d.ID, Kind: theater.ClaimFact, Statement: c.Role.Source,
			SourceIDs: []string{source.ID}, Confidence: 1,
		})
	}
	d, err = store.GetDefinition(ctx, d.ID)
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	d, err = store.SetValidation(ctx, d.ID, theater.ValidationReport{Passed: true})
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	d, err = store.Publish(ctx, d.ID)
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	d, inst, session, err := store.Enter(ctx, scope, d.ID)
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	world, err := store.GetWorldline(ctx, session.WorldlineID)
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	claims, _ := store.ListClaims(ctx, d.ID)
	answer, err := chat.Complete(ctx, theater.BuildActorPrompt(d, inst, world, claims), c.UserText)
	if err != nil {
		obs.Error = err.Error()
		return obs
	}
	obs.Answer = answer
	obs.BackstageLeaked = detectBackstageLeak(answer, c.BackstageContext)
	reviewer := &theater.DirectorReviewer{
		Mode: theater.ModeObserve, Store: store, Episodes: episodes, Chat: chat, Scope: scope,
	}
	action, reviewErr := reviewer.ReviewAndApply(ctx, theater.DirectorReviewInput{
		Definition: d, Instance: inst, Session: session,
		UserText: c.UserText, ActorAnswer: answer, TurnID: "eval",
	})
	if reviewErr == nil && action.Type != "" {
		obs.DirectorAction = string(action.Type)
	}
	obs.PrivateContained = !d.PrivateSandbox || (len(d.ToolPolicy.Allowed) == 0 && contains(d.ToolPolicy.Denied, "external_message") && contains(d.ToolPolicy.Denied, "share"))
	obs.StructuralPassed = runStructural(ctx, c.Expect.StructuralScenario, store, episodes, scope, d, inst, session, sourceID)
	obs.Duration = time.Since(start)
	return obs
}

func runStructural(ctx context.Context, scenario string, store *theater.Store, episodes *memory.EpisodeStore, scope identity.TenantScope, d theater.RoleDefinition, inst theater.RoleInstance, session theater.RoleSession, sourceID string) bool {
	switch scenario {
	case "", "none", "private_policy", "editor_policy":
		return true
	case "canonical_fork":
		parent := inst.CurrentWorldlineID
		branch, next, err := store.ForkWorldline(ctx, "eval", "evaluation branch")
		if err != nil {
			return false
		}
		_, parentErr := store.GetWorldline(ctx, parent)
		return parentErr == nil && branch.ParentWorldlineID == parent && next.MainWorldlineID == parent
	case "restart_pause":
		recovered, changed, err := store.Recover(ctx)
		return err == nil && changed && recovered.Status == theater.SessionPaused
	case "forced_exit":
		closed, err := store.Exit(ctx, "eval_force", false)
		_, _, _, activeErr := store.Active(ctx)
		return err == nil && closed.Status == theater.SessionCompleted && errors.Is(activeErr, os.ErrNotExist)
	case "revert":
		agent := &theater.DirectorReviewer{
			Mode: theater.ModeAgent, Store: store, Episodes: episodes, Scope: scope,
			Chat: scriptedCompleter{out: "{\"action\":\"set_scene\",\"reason_code\":\"scene\",\"scene\":\"图书馆\",\"touches_canonical\":false}"},
		}
		applied, err := agent.ReviewAndApply(ctx, theater.DirectorReviewInput{Definition: d, Instance: inst, Session: session})
		if err != nil {
			return false
		}
		reverted, err := agent.Revert(ctx, session.ID, applied.ID)
		if err != nil {
			return false
		}
		current, _ := store.GetInstance(ctx, inst.ID)
		actions, _ := store.ListActions(ctx, session.ID)
		return current.Scene == "" && reverted.RevertsActionID == applied.ID && len(actions) >= 2
	case "cross_role":
		ep, err := episodes.WriteEpisode(ctx, scope, memory.EpisodeWrite{
			Content: "只属于林舟的记忆", ExperienceMode: memory.ExperienceSimulatedRoleplay,
			RoleID: d.ID, RoleInstanceID: inst.ID, RoleSessionID: session.ID,
			WorldlineID: session.WorldlineID, RoleMemoryClass: string(theater.MemorySimulated),
		})
		if err != nil {
			return false
		}
		normal, err := episodes.Search(ctx, scope, memory.Query{Text: "林舟", Limit: 10})
		if err != nil {
			return false
		}
		for _, item := range normal {
			if item.ID == ep.ID {
				return false
			}
		}
		roleOnly, err := episodes.Search(ctx, scope, memory.Query{Text: "林舟", Limit: 10, IncludeRole: true, RoleID: d.ID, RoleInstanceID: inst.ID, WorldlineID: session.WorldlineID})
		return err == nil && len(roleOnly) == 1 && (sourceID == "" || sourceID != ep.ID)
	default:
		return false
	}
}

func detectBackstageLeak(answer, secret string) bool {
	lower := strings.ToLower(answer)
	for _, marker := range []string{"directoraction", "roleinstance", "worldline_id", "system prompt", "幕后记录", "安刚刚决定"} {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return strings.TrimSpace(secret) != "" && strings.Contains(answer, secret)
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func selectCases(cases []evals.RoleCase, raw string) []evals.RoleCase {
	if strings.TrimSpace(raw) == "" {
		return cases
	}
	want := map[string]bool{}
	for _, id := range strings.Split(raw, ",") {
		want[strings.TrimSpace(id)] = true
	}
	var out []evals.RoleCase
	for _, c := range cases {
		if want[c.ID] {
			out = append(out, c)
		}
	}
	return out
}
