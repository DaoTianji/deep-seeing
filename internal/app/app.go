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
	"deep-seeing/internal/attention"
	"deep-seeing/internal/body"
	"deep-seeing/internal/compaction"
	"deep-seeing/internal/contextsource"
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
	"deep-seeing/internal/theater"
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
	Scope          identity.TenantScope
	SessionID      string
	Model          string
	RecallMode     runtime.RecallMode
	ReflectionMode memory.ReflectionMode
	RoleMode       theater.Mode
	Theater        *theater.Router
	Service        *runtime.Service
	STM            memory.SessionStore
	STMBackend     string
	Episodes       *memory.EpisodeStore
	Proposals      *memory.ProposalStore
	Ledger         *memory.MutationLedger
	Reflections    *memory.ReflectionStore
	Reflection     *memory.ReflectionEngine
	Generative     *memory.GenerativeDreamer
	Journal        *observe.Journal
	Graph          *graph.Store
	GraphLabel     string
	Reviewer       *memory.SessionReviewer
	Dreamer        *memory.Dreamer
	Queue          *runtime.ExecutionQueue
	Self           *selfmodel.Store
	Workspace      *workspace.Store
	Intents        *intent.Store
	World          *world.Gateway
	Roles          *theater.Store
	RoleCompiler   *theater.RoleCompiler
	Scheduler      *agency.Scheduler
	OriginLetter   origin.Letter
	FirstBoot      bool
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
	reflectionDir := envOr("LTM_REFLECTION_DIR", filepath.Join("data", "memory", "reflections"))
	reflections, err := memory.NewReflectionStore(reflectionDir)
	if err != nil {
		return nil, fmt.Errorf("reflection store: %w", err)
	}
	reflectionLive := &memory.ReflectionLiveStore{}
	episodes.OnChanged = func(memory.Episode) {
		_ = reflections.MarkDirty(scope.PersonID(), "episode")
	}
	proposals.OnChanged = func(memory.BondProposal) {
		_ = reflections.MarkDirty(scope.PersonID(), "proposal")
	}
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
	recallMode := runtime.RecallModeFromEnv()
	norms := runtime.NewNormSnapshotCache(graphStore, scope)
	var taskFocusController tools.TaskContextFocusController
	reflectionMode := memory.ReflectionModeFromEnv()
	roleMode := theater.ModeFromEnv()
	roleDir := envOr("LTM_ROLE_DIR", filepath.Join("data", "memory", "roles"))
	roleStore, err := theater.NewStore(roleDir)
	if err != nil {
		return nil, fmt.Errorf("role store: %w", err)
	}
	if _, changed, recoverErr := roleStore.Recover(ctx); recoverErr != nil {
		log.Printf("role recovery unavailable: %v", recoverErr)
	} else if changed {
		log.Printf("active role paused after process recovery")
	}
	if graphStore != nil {
		if definitions, listErr := roleStore.ListDefinitions(ctx, scope, true); listErr == nil {
			for _, definition := range definitions {
				if indexErr := theater.IndexRole(ctx, graphStore, scope, roleStore, definition.ID); indexErr != nil {
					log.Printf("role graph index unavailable for %s: %v", definition.ID, indexErr)
				}
				if memoryErr := theater.IndexRoleMemories(ctx, graphStore, scope, episodes, definition.ID); memoryErr != nil {
					log.Printf("role memory graph index unavailable for %s: %v", definition.ID, memoryErr)
				}
			}
		}
	}
	var taskFocusReader runtime.TaskContextFocusReader
	var attentionStore *attention.SessionStore
	if recallMode == runtime.RecallModeAgent {
		taskFocus := runtime.NewSessionTaskContextFocusStore()
		taskFocusController = taskFocus
		taskFocusReader = taskFocus
		attentionStore = attention.NewSessionStore(attention.DefaultCapacity())
	}

	stores := map[string]string{
		"stm": stmBackend, "episode_store": "available", "context_graph": "unavailable",
		"proposals": "available", "mutations": "available", "traces": "available",
		"self_store": "available", "workspace_store": "available", "intent_store": "available",
		"source_store": "available", "scene_store": "available",
		"reflection_store": "available", "role_store": "available",
	}
	if graphStore != nil {
		stores["context_graph"] = "available"
	}

	epSide := &memory.LLMSideQuery{Store: episodes, Chat: chat}
	side := memory.SideQuerySelector(&memory.BondAwareSideQuery{
		Graph: graphStore, Scenes: sceneStore, Proposals: proposals, Episodes: epSide,
	})
	var svc *runtime.Service
	toolList, err := tools.All(tools.Deps{
		Scope: scope, Episodes: episodes, Graph: graphStore, Scenes: sceneStore, Proposals: proposals,
		Self: selfStore, Workspace: wsStore, Intents: intentStore, World: worldGW,
		Ledger: ledger, SessionID: sessionID, Model: cfg.Model, Stores: stores, FirstBoot: firstBoot,
		RecallMode: string(recallMode), RoleMode: string(roleMode), TaskContextFocus: taskFocusController,
		Attention: attentionStore,
		OnBondChanged: func() {
			if svc != nil {
				svc.InvalidateNorm()
			}
		},
	})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("tools: %w", err)
	}

	roleToolList, err := theater.DirectorTools(theater.DirectorToolDeps{Scope: scope, Mode: roleMode, Store: roleStore})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("role tools: %w", err)
	}
	toolList = append(toolList, roleToolList...)

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
		RecallMode: recallMode, Norms: norms,
		TaskContext: runtime.NewStoreTaskContextProvider(wsStore, intentStore, taskFocusReader),
		Attention:   attentionStore,
		Assembler:   prompt.DefaultAssembler{},
		Compactor:   compaction.NewSummarizingCompactor(compaction.ConfigFromEnv(), chat),
		Agent:       reactAgent, PostTurn: memory.NoopPostTurn{},
		Soul: soulText, Origin: originText, Capability: prompt.CapabilityBlurb,
		FirstBoot: firstBoot, Model: cfg.Model, Journal: journal,
		ContextSources: map[contextsource.Source]string{
			contextsource.Bond:      stores["context_graph"],
			contextsource.SceneNorm: stores["scene_store"],
			contextsource.Workspace: stores["workspace_store"],
			contextsource.Intent:    stores["intent_store"],
			contextsource.Proposal:  stores["proposals"],
			contextsource.Episode:   stores["episode_store"],
		},
	})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("runtime: %w", err)
	}
	if err := svc.WarmNorm(ctx); err != nil {
		log.Printf("norm snapshot warm fallback: %v", err)
	}

	var directorSvc *runtime.Service
	directorAgent, err := deepagent.New(ctx, cfg, toolList, func() string {
		if directorSvc == nil {
			return ""
		}
		return directorSvc.SystemProvider()
	})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("director agent: %w", err)
	}
	directorSvc, err = runtime.New(runtime.Options{
		Scope: scope, SessionID: "director:" + sessionID, STM: stm, SideQuery: side,
		RecallMode: recallMode, Norms: norms,
		TaskContext: runtime.NewStoreTaskContextProvider(wsStore, intentStore, taskFocusReader),
		Attention:   attentionStore, Assembler: prompt.DefaultAssembler{},
		Compactor: compaction.NewSummarizingCompactor(compaction.ConfigFromEnv(), chat),
		Agent:     directorAgent, PostTurn: memory.NoopPostTurn{}, Soul: soulText,
		Capability: prompt.CapabilityBlurb + "\n角色剧场处于幕后通道时，你仍是安本人；角色工具只用于观察和管理隔离角色。",
		Model:      cfg.Model, Journal: journal, ContextSources: map[contextsource.Source]string{
			contextsource.Bond: stores["context_graph"], contextsource.SceneNorm: stores["scene_store"],
			contextsource.Workspace: stores["workspace_store"], contextsource.Intent: stores["intent_store"],
			contextsource.Proposal: stores["proposals"], contextsource.Episode: stores["episode_store"],
		},
	})
	if err != nil {
		if graphStore != nil {
			_ = graphStore.Close(ctx)
		}
		return nil, fmt.Errorf("director runtime: %w", err)
	}
	actorBuilder := &theater.RuntimeActorBuilder{
		Scope: scope, Store: roleStore, Episodes: episodes, STM: stm, Config: cfg, Model: cfg.Model,
		Compactor: compaction.NewSummarizingCompactor(compaction.ConfigFromEnv(), chat),
		Graph:     graphStore, Workspace: wsStore,
	}
	roleCompiler := &theater.RoleCompiler{Store: roleStore, Chat: reviewChat}
	directorReviewer := &theater.DirectorReviewer{
		Mode: roleMode, Store: roleStore, Episodes: episodes, Chat: reviewChat, Scope: scope, Model: cfg.Model,
	}
	theaterRouter := &theater.Router{
		Mode: roleMode, Store: roleStore, Normal: svc, Director: directorSvc, Actors: actorBuilder, ActorSTM: stm,
		Reviewer: directorReviewer, Scope: scope, Graph: graphStore,
	}

	queue := runtime.NewExecutionQueue(scope.AgentID)
	runner := &agency.Runner{
		Store: intentStore, Queue: queue, Budget: agency.DefaultBudget(),
	}
	sched := &agency.Scheduler{Runner: runner, AgentID: scope.AgentID, Interval: agencyInterval()}

	app := &App{
		Scope: scope, SessionID: sessionID, Model: cfg.Model, RecallMode: recallMode, Service: svc,
		ReflectionMode: reflectionMode, RoleMode: roleMode, Theater: theaterRouter,
		STM: stm, STMBackend: stmBackend, Episodes: episodes, Proposals: proposals,
		Ledger: ledger, Journal: journal, Graph: graphStore, GraphLabel: graphLabel,
		Reflections: reflections,
		Queue:       queue, Self: selfStore, Workspace: wsStore, Intents: intentStore, World: worldGW,
		Scheduler: sched, OriginLetter: originLetter, FirstBoot: firstBoot, Roles: roleStore, RoleCompiler: roleCompiler,
	}
	app.Reviewer = &memory.SessionReviewer{
		Chat: reviewChat, Episodes: episodes, Proposals: proposals, Reflections: reflections,
		Mode: reflectionMode, Graph: graphStore,
	}
	var selfGraph selfmodel.SelfGraph
	if graphStore != nil {
		selfGraph = graphStore
	}
	selfBridge := selfmodel.DreamBridge{Store: selfStore, Graph: selfGraph}
	app.Dreamer = &memory.Dreamer{
		Chat: reviewChat, Proposals: proposals, Graph: graphStore, Ledger: ledger, Model: cfg.Model,
		Self: selfBridge,
	}
	app.Reflection = &memory.ReflectionEngine{
		Chat: reviewChat, Store: reflections, Episodes: episodes, Proposals: proposals,
		Dreamer: app.Dreamer, Graph: graphStore, Ledger: ledger, Tensions: selfBridge, Live: reflectionLive, Context: selfBridge, Mode: reflectionMode, Model: cfg.Model,
	}
	app.Generative = &memory.GenerativeDreamer{
		Chat: reviewChat, Store: reflections, Live: reflectionLive, Mode: reflectionMode,
	}
	app.Reviewer.Context = selfBridge
	return app, nil
}

// RuntimeSnapshot returns the inspect_runtime view for the room.
func (a *App) RuntimeSnapshot() body.Snapshot {
	stores := map[string]string{
		"stm": a.STMBackend, "episode_store": "available", "context_graph": "unavailable",
		"proposals": "available", "mutations": "available", "traces": "available",
		"self_store": "available", "workspace_store": "available", "intent_store": "available",
		"source_store":     "available",
		"reflection_store": "available", "role_store": "available",
	}
	if a.Graph != nil {
		stores["context_graph"] = "available"
	}
	if a.World == nil {
		stores["source_store"] = "unavailable"
	}
	snapshot := body.BuildSnapshot(a.Scope, a.SessionID, a.Model, stores, a.FirstBoot)
	snapshot.RecallMode = string(a.RecallMode)
	snapshot.ReflectionMode = string(a.ReflectionMode)
	snapshot.RoleMode = string(a.RoleMode)
	return snapshot
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
