// Package app assembles the Deep-Seeing runtime for CLI and embedded room.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"deep-seeing/internal/agency"
	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/body"
	"deep-seeing/internal/compaction"
	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/intent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/origin"
	"deep-seeing/internal/prompt"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/selfmodel"
	"deep-seeing/internal/soul"
	"deep-seeing/internal/tools"
	"deep-seeing/internal/workspace"
	"deep-seeing/internal/world"
)

// Options controls one assembled runtime.
type Options struct {
	SessionID string
}

// App owns the long-lived services shared by CLI and room.
type App struct {
	Scope        identity.TenantScope
	SessionID    string
	Model        string
	Service      *runtime.Service
	STM          memory.SessionStore
	STMBackend   string
	Episodes     *memory.EpisodeStore
	Proposals    *memory.ProposalStore
	Ledger       *memory.MutationLedger
	Journal      *observe.Journal
	Graph        *graph.Store
	GraphLabel   string
	Reviewer     *memory.SessionReviewer
	Dreamer      *memory.Dreamer
	Queue        *runtime.ExecutionQueue
	Self         *selfmodel.Store
	Workspace    *workspace.Store
	Intents      *intent.Store
	World        *world.Gateway
	Scheduler    *agency.Scheduler
	OriginLetter origin.Letter
	FirstBoot    bool
}

// New loads environment settings and assembles a complete application.
func New(ctx context.Context, opt Options) (*App, error) {
	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")

	scope := identity.LocalCLI()
	sessionID := strings.TrimSpace(opt.SessionID)
	if sessionID == "" {
		sessionID = "room"
	}
	cfg := deepagent.ConfigFromEnv()

	soulText := soul.MustLoad(os.Getenv("SOUL_PATH"))
	originLetter, err := origin.LoadForScope(os.Getenv("ORIGIN_DIR"), scope)
	if err != nil {
		log.Printf("origin context unavailable: %v", err)
	}
	forceOrigin := envBool("FORCE_ORIGIN")
	originText, firstBoot, err := origin.IntroductionForBoot(
		origin.BootGate{StateDir: strings.TrimSpace(os.Getenv("LTM_STATE_DIR"))},
		scope, originLetter, forceOrigin,
	)
	if err != nil {
		log.Printf("origin boot gate: %v", err)
	}

	epDir := envOr("LTM_EPISODE_DIR", filepath.Join("data", "memory", "episodes"))
	episodes, err := memory.NewEpisodeStore(epDir)
	if err != nil {
		return nil, fmt.Errorf("episode store: %w", err)
	}
	propDir := envOr("LTM_PROPOSAL_DIR", filepath.Join("data", "memory", "proposals"))
	proposals, err := memory.NewProposalStore(propDir)
	if err != nil {
		return nil, fmt.Errorf("proposal store: %w", err)
	}
	mutDir := envOr("LTM_MUTATION_DIR", filepath.Join("data", "memory", "mutations"))
	ledger, err := memory.NewMutationLedger(mutDir)
	if err != nil {
		return nil, fmt.Errorf("mutation ledger: %w", err)
	}
	selfDir := envOr("LTM_SELF_DIR", filepath.Join("data", "memory", "self"))
	selfStore, err := selfmodel.NewStore(selfDir)
	if err != nil {
		return nil, fmt.Errorf("self store: %w", err)
	}
	wsDir := envOr("LTM_WORKSPACE_DIR", filepath.Join("data", "memory", "workspace"))
	wsStore, err := workspace.NewStore(wsDir)
	if err != nil {
		return nil, fmt.Errorf("workspace store: %w", err)
	}
	rtDir := envOr("LTM_RUNTIME_DIR", filepath.Join("data", "runtime"))
	intentStore, err := intent.OpenStore(rtDir)
	if err != nil {
		return nil, fmt.Errorf("intent store: %w", err)
	}
	srcDir := envOr("LTM_SOURCE_DIR", filepath.Join("data", "memory", "sources"))
	worldGW, err := world.NewGateway(srcDir)
	if err != nil {
		return nil, fmt.Errorf("world gateway: %w", err)
	}
	sceneDir := envOr("LTM_SCENE_DIR", filepath.Join("data", "memory", "scenes"))
	sceneStore, err := memory.NewSceneStore(sceneDir)
	if err != nil {
		return nil, fmt.Errorf("scene store: %w", err)
	}
	traceDir := envOr("LTM_TRACE_DIR", filepath.Join("data", "memory", "traces"))
	journal, err := observe.NewJournal(traceDir)
	if err != nil {
		log.Printf("trace journal unavailable: %v", err)
	}

	chat := &memory.ChatClient{
		APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 512,
	}
	reviewChat := &memory.ChatClient{
		APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 1024,
	}

	stm, stmBackend := openSTM(ctx, scope)
	graphStore, graphLabel := openGraph(ctx, scope)

	stores := map[string]string{
		"stm": stmBackend, "episode_store": "available", "context_graph": "unavailable",
		"proposals": "available", "mutations": "available", "traces": "available",
		"self_store": "available", "workspace_store": "available", "intent_store": "available",
		"source_store": "available", "scene_store": "available",
	}
	if graphStore != nil {
		stores["context_graph"] = "available"
	}

	epSide := &memory.LLMSideQuery{Store: episodes, Chat: chat}
	side := memory.SideQuerySelector(&memory.BondAwareSideQuery{
		Graph: graphStore, Scenes: sceneStore, Proposals: proposals, Episodes: epSide,
	})
	toolList, err := tools.All(tools.Deps{
		Scope: scope, Episodes: episodes, Graph: graphStore, Scenes: sceneStore, Proposals: proposals,
		Self: selfStore, Workspace: wsStore, Intents: intentStore, World: worldGW,
		Ledger: ledger, SessionID: sessionID, Model: cfg.Model, Stores: stores, FirstBoot: firstBoot,
	})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("tools: %w", err)
	}

	var svc *runtime.Service
	reactAgent, err := deepagent.New(ctx, cfg, toolList, func() string {
		if svc == nil {
			return ""
		}
		return svc.SystemProvider()
	})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("agent: %w", err)
	}
	svc, err = runtime.New(runtime.Options{
		Scope: scope, SessionID: sessionID, STM: stm, SideQuery: side,
		Assembler: prompt.DefaultAssembler{},
		Compactor: compaction.NewSummarizingCompactor(compaction.ConfigFromEnv(), chat),
		Agent:     reactAgent, PostTurn: memory.NoopPostTurn{},
		Soul: soulText, Origin: originText, Capability: prompt.CapabilityBlurb,
		FirstBoot: firstBoot, Model: cfg.Model, Journal: journal,
	})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("runtime: %w", err)
	}

	queue := runtime.NewExecutionQueue(scope.AgentID)
	runner := &agency.Runner{
		Store: intentStore, Queue: queue, Budget: agency.DefaultBudget(),
	}
	sched := &agency.Scheduler{Runner: runner, AgentID: scope.AgentID, Interval: agencyInterval()}

	app := &App{
		Scope: scope, SessionID: sessionID, Model: cfg.Model, Service: svc,
		STM: stm, STMBackend: stmBackend, Episodes: episodes, Proposals: proposals,
		Ledger: ledger, Journal: journal, Graph: graphStore, GraphLabel: graphLabel,
		Queue: queue, Self: selfStore, Workspace: wsStore, Intents: intentStore, World: worldGW,
		Scheduler: sched, OriginLetter: originLetter, FirstBoot: firstBoot,
	}
	app.Reviewer = &memory.SessionReviewer{
		Chat: reviewChat, Episodes: episodes, Proposals: proposals, Graph: graphStore,
	}
	var selfGraph selfmodel.SelfGraph
	if graphStore != nil {
		selfGraph = graphStore
	}
	app.Dreamer = &memory.Dreamer{
		Chat: reviewChat, Proposals: proposals, Graph: graphStore, Ledger: ledger, Model: cfg.Model,
		Self: selfmodel.DreamBridge{Store: selfStore, Graph: selfGraph},
	}
	return app, nil
}

// RuntimeSnapshot returns the inspect_runtime view for the room.
func (a *App) RuntimeSnapshot() body.Snapshot {
	stores := map[string]string{
		"stm": a.STMBackend, "episode_store": "available", "context_graph": "unavailable",
		"proposals": "available", "mutations": "available", "traces": "available",
		"self_store": "available", "workspace_store": "available", "intent_store": "available",
		"source_store": "available",
	}
	if a.Graph != nil {
		stores["context_graph"] = "available"
	}
	if a.World == nil {
		stores["source_store"] = "unavailable"
	}
	return body.BuildSnapshot(a.Scope, a.SessionID, a.Model, stores, a.FirstBoot)
}

// StartScheduler starts the agency wake loop (daemon mode).
func (a *App) StartScheduler(ctx context.Context) {
	if a == nil || a.Scheduler == nil {
		return
	}
	a.Scheduler.Start(ctx)
}

// Close releases remote clients.
func (a *App) Close(ctx context.Context) {
	if a == nil {
		return
	}
	if a.Scheduler != nil {
		a.Scheduler.Stop()
	}
	if a.Intents != nil {
		_ = a.Intents.Close()
	}
	if a.Graph != nil {
		_ = a.Graph.Close(ctx)
	}
	if closer, ok := a.STM.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func openGraph(ctx context.Context, scope identity.TenantScope) (*graph.Store, string) {
	store, err := graph.OpenFromEnv(ctx)
	if err != nil {
		log.Printf("LTM graph unavailable, episodes only: %v", err)
		return nil, "episodes (graph offline)"
	}
	if store == nil {
		return nil, "episodes"
	}
	if err := store.EnsureSchema(ctx); err != nil {
		_ = store.Close(ctx)
		return nil, "episodes (graph schema failed)"
	}
	if err := store.EnsureOriginSeed(ctx, scope, origin.RoleAtOrigin); err != nil {
		_ = store.Close(ctx)
		return nil, "episodes (graph seed failed)"
	}
	return store, "episodes+graph"
}

func openSTM(ctx context.Context, scope identity.TenantScope) (memory.SessionStore, string) {
	maxMsg := memory.STMMaxMessagesFromEnv()
	rdb, err := memory.NewRedisSTMFromEnv(ctx, scope)
	if err != nil {
		log.Printf("STM redis unavailable, fallback to memory: %v", err)
		return memory.NewSTM(maxMsg), "memory"
	}
	rdb.MaxMessages = maxMsg
	return rdb, "redis"
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "on"
}

func agencyInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AGENCY_TICK"))
	if raw == "" {
		return time.Minute
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 5*time.Second {
		return time.Minute
	}
	return d
}
