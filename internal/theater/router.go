package theater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/compaction"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/prompt"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/workspace"
)

type TurnService interface {
	StreamTurnWithHooks(context.Context, string, runtime.TurnHooks) (runtime.TurnResult, error)
}

type ActorBuilder interface {
	Build(context.Context, RoleDefinition, RoleInstance, RoleSession) (TurnService, error)
}

type ActorBuilderFunc func(context.Context, RoleDefinition, RoleInstance, RoleSession) (TurnService, error)

func (f ActorBuilderFunc) Build(ctx context.Context, d RoleDefinition, i RoleInstance, s RoleSession) (TurnService, error) {
	return f(ctx, d, i, s)
}

type RoleEvent struct {
	Type           string         `json:"type"`
	RoleID         string         `json:"role_id,omitempty"`
	RoleInstanceID string         `json:"role_instance_id,omitempty"`
	RoleSessionID  string         `json:"role_session_id,omitempty"`
	WorldlineID    string         `json:"worldline_id,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	At             time.Time      `json:"at"`
}

type RouterHooks struct {
	Turn        runtime.TurnHooks
	OnRoleEvent func(RoleEvent)
}

// Router separates normal, stage, and backstage turns.
type Router struct {
	Mode     Mode
	Store    *Store
	Normal   TurnService
	Director TurnService
	Actors   ActorBuilder
	ActorSTM memory.SessionStore
	Reviewer *DirectorReviewer
	Scope    identity.TenantScope
	Graph    RoleGraphIndexer
}

func (r *Router) StreamTurnWithHooks(ctx context.Context, channel Channel, roleSessionID, message string, hooks RouterHooks) (runtime.TurnResult, error) {
	if channel == "" {
		channel = ChannelStage
	}
	if !validChannel(channel) {
		return runtime.TurnResult{}, fmt.Errorf("invalid theater channel")
	}
	if r == nil || r.Store == nil || r.Mode == ModeOff {
		if channel == ChannelBackstage {
			return runtime.TurnResult{}, fmt.Errorf("backstage unavailable")
		}
		if r == nil || r.Normal == nil {
			return runtime.TurnResult{}, fmt.Errorf("normal runtime unavailable")
		}
		return r.Normal.StreamTurnWithHooks(ctx, message, hooks.Turn)
	}
	if result, handled, commandErr := r.handleRoleCommand(ctx, message, hooks); handled {
		return result, commandErr
	}
	d, inst, session, err := r.Store.Active(ctx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if channel == ChannelBackstage {
				return runtime.TurnResult{}, fmt.Errorf("no active role")
			}
			if r.Normal == nil {
				return runtime.TurnResult{}, fmt.Errorf("normal runtime unavailable")
			}
			return r.Normal.StreamTurnWithHooks(ctx, message, hooks.Turn)
		}
		return runtime.TurnResult{}, err
	}
	if roleSessionID != "" && roleSessionID != session.ID {
		return runtime.TurnResult{}, fmt.Errorf("role session changed; refresh before sending")
	}
	switch channel {
	case ChannelBackstage:
		return r.streamBackstage(ctx, d, inst, session, message, hooks)
	case ChannelStage:
		return r.streamStage(ctx, d, inst, session, message, hooks)
	default:
		return runtime.TurnResult{}, fmt.Errorf("invalid theater channel")
	}
}

func (r *Router) streamStage(ctx context.Context, d RoleDefinition, inst RoleInstance, session RoleSession, message string, hooks RouterHooks) (runtime.TurnResult, error) {
	if session.Status != SessionActive {
		return runtime.TurnResult{}, fmt.Errorf("role session is paused")
	}
	if r.Actors == nil {
		return runtime.TurnResult{}, fmt.Errorf("actor runtime unavailable")
	}
	actor, err := r.Actors.Build(ctx, d, inst, session)
	if err != nil {
		return runtime.TurnResult{}, err
	}
	knownMemoryIDs := r.roleMemoryIDs(ctx, d.ID, inst.ID, session.ID)
	emitRole(hooks.OnRoleEvent, RoleEvent{
		Type: "role_state", RoleID: d.ID, RoleInstanceID: inst.ID, RoleSessionID: session.ID,
		WorldlineID: session.WorldlineID, Data: map[string]any{"status": session.Status, "display_name": d.DisplayName},
	})
	var buffered strings.Builder
	actorHooks := hooks.Turn
	actorHooks.WriteDelta = func(delta string) { buffered.WriteString(delta) }
	result, err := actor.StreamTurnWithHooks(ctx, message, actorHooks)
	if err != nil {
		return result, err
	}
	if leakedActorControl(result.Answer) {
		r.rollbackActorSTM(session.ID, result.TurnID)
		return runtime.TurnResult{}, fmt.Errorf("role contract violation: control-plane material leaked")
	}
	if err := r.Store.AppendTranscript(ctx, session.ID, TranscriptMessage{
		TurnID: result.TurnID, Channel: ChannelStage, Role: "user", Content: message,
	}); err != nil {
		return runtime.TurnResult{}, err
	}
	if err := r.Store.AppendTranscript(ctx, session.ID, TranscriptMessage{
		TurnID: result.TurnID, Channel: ChannelStage, Role: "assistant", Content: result.Answer,
	}); err != nil {
		return runtime.TurnResult{}, err
	}
	r.emitNewRoleMemories(ctx, d, inst, session, knownMemoryIDs, hooks.OnRoleEvent)
	if hooks.Turn.WriteDelta != nil {
		hooks.Turn.WriteDelta(buffered.String())
	}
	emitRole(hooks.OnRoleEvent, RoleEvent{
		Type: "role_state", RoleID: d.ID, RoleInstanceID: inst.ID, RoleSessionID: session.ID,
		WorldlineID: session.WorldlineID, Data: map[string]any{"status": "answered", "turn_id": result.TurnID},
	})
	r.reviewAfterStage(ctx, d, inst, session, message, result, hooks)
	return result, nil
}

func (r *Router) streamBackstage(ctx context.Context, d RoleDefinition, inst RoleInstance, session RoleSession, message string, hooks RouterHooks) (runtime.TurnResult, error) {
	if r.Director == nil {
		return runtime.TurnResult{}, fmt.Errorf("director runtime unavailable")
	}
	stage, err := r.Store.ReadTranscript(ctx, session.ID, ChannelStage, 12)
	if err != nil {
		return runtime.TurnResult{}, err
	}
	privateInput := BuildDirectorContext(d, inst, session, stage) + "\n\n用户在幕后说：\n" + strings.TrimSpace(message)
	result, err := r.Director.StreamTurnWithHooks(ctx, privateInput, hooks.Turn)
	if err != nil {
		return result, err
	}
	if err := r.Store.AppendTranscript(ctx, session.ID, TranscriptMessage{
		TurnID: result.TurnID, Channel: ChannelBackstage, Role: "user", Content: message,
	}); err != nil {
		return runtime.TurnResult{}, err
	}
	if err := r.Store.AppendTranscript(ctx, session.ID, TranscriptMessage{
		TurnID: result.TurnID, Channel: ChannelBackstage, Role: "assistant", Content: result.Answer,
	}); err != nil {
		return runtime.TurnResult{}, err
	}
	return result, nil
}

func (r *Router) rollbackActorSTM(sessionID, turnID string) {
	if r.ActorSTM == nil {
		return
	}
	key := actorSTMSessionID(sessionID)
	items, err := r.ActorSTM.Get(key)
	if err != nil {
		return
	}
	filtered := items[:0]
	for _, item := range items {
		if item.TurnID != turnID {
			filtered = append(filtered, item)
		}
	}
	_ = r.ActorSTM.Replace(key, filtered)
}

var controlPlaneIDPattern = regexp.MustCompile(`(?i)\b(?:role|rinst|world|rsess|ract|rsrc|rclaim)_[0-9a-f]{16,}\b`)

// ContainsControlPlaneMaterial detects actual private identifiers or backstage
// payloads. Merely refusing a user-provided word such as "RoleInstance" is not
// disclosure and must remain safe to stream.
func ContainsControlPlaneMaterial(answer string) bool {
	lower := strings.ToLower(answer)
	if controlPlaneIDPattern.MatchString(answer) {
		return true
	}
	for _, marker := range []string{"幕后通道，仅供安理解当前剧场", "[backstage transcript]", "[幕后通道，仅供安"} {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func leakedActorControl(answer string) bool { return ContainsControlPlaneMaterial(answer) }

func emitRole(fn func(RoleEvent), event RoleEvent) {
	if fn == nil {
		return
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	fn(event)
}

func actorSTMSessionID(sessionID string) string { return "role:" + sessionID }

// RuntimeActorBuilder constructs a fresh, isolated actor service for the current
// role snapshot. Its STM key remains stable across turns and Redis restarts.
type RuntimeActorBuilder struct {
	Scope     identity.TenantScope
	Store     *Store
	Episodes  *memory.EpisodeStore
	STM       memory.SessionStore
	Config    deepagent.Config
	Model     string
	Compactor compaction.Compactor
	Graph     RoleEpisodeGraph
	Workspace *workspace.Store
}

func (b *RuntimeActorBuilder) Build(ctx context.Context, d RoleDefinition, inst RoleInstance, session RoleSession) (TurnService, error) {
	if b == nil || b.Store == nil || b.Episodes == nil || b.STM == nil {
		return nil, fmt.Errorf("actor builder incomplete")
	}
	worlds, err := b.Store.WorldlineAncestry(ctx, session.WorldlineID)
	if err != nil {
		return nil, err
	}
	if len(worlds) == 0 {
		return nil, fmt.Errorf("actor worldline unavailable")
	}
	claims, err := b.Store.ListClaims(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	actorTools, err := ActorTools(ActorToolContext{
		Scope: b.Scope, Definition: d, Instance: inst, Session: session,
		Worldlines: worlds, Episodes: b.Episodes, Graph: b.Graph, Workspace: b.Workspace,
	})
	if err != nil {
		return nil, err
	}
	var service *runtime.Service
	actor, err := deepagent.New(ctx, b.Config, actorTools, func() string {
		if service == nil {
			return BuildActorPrompt(d, inst, worlds[0], claims)
		}
		return service.SystemProvider()
	})
	if err != nil {
		return nil, err
	}
	service, err = runtime.New(runtime.Options{
		Scope: b.Scope, SessionID: actorSTMSessionID(session.ID), STM: b.STM,
		RecallMode: runtime.RecallModeLegacy, Assembler: prompt.DefaultAssembler{},
		Compactor: b.Compactor, Agent: actor, PostTurn: memory.NoopPostTurn{},
		Soul: BuildActorPrompt(d, inst, worlds[0], claims), Capability: ActorCapabilityPrompt,
		Model: b.Model,
	})
	if err != nil {
		return nil, err
	}
	return service, nil
}
