package theater

import (
	"context"

	"deep-seeing/internal/runtime"
)

func (r *Router) reviewAfterStage(ctx context.Context, d RoleDefinition, inst RoleInstance, session RoleSession, message string, result runtime.TurnResult, hooks RouterHooks) {
	if r.Reviewer == nil {
		return
	}
	action, err := r.Reviewer.ReviewAndApply(ctx, DirectorReviewInput{
		Definition: d, Instance: inst, Session: session, UserText: message,
		ActorAnswer: result.Answer, TurnID: result.TurnID,
	})
	if r.Graph != nil {
		_ = IndexRole(ctx, r.Graph, r.Scope, r.Store, d.ID)
		if memoryID := action.After["memory_id"]; memoryID != "" && r.Reviewer.Episodes != nil {
			if ep, getErr := r.Reviewer.Episodes.Get(ctx, memoryID); getErr == nil {
				_ = IndexRoleEpisode(ctx, r.Graph, r.Scope, ep)
			}
		}
		if memoryID := action.After["memory_id"]; memoryID != "" {
			emitRole(hooks.OnRoleEvent, RoleEvent{Type: "role_memory_written", RoleID: d.ID, RoleInstanceID: inst.ID, RoleSessionID: session.ID, WorldlineID: action.WorldlineID, Data: map[string]any{"episode_id": memoryID, "source": "director"}})
		}
	}
	data := map[string]any{
		"turn_id":             result.TurnID,
		"action":              action.Type,
		"status":              action.Status,
		"reason_code":         action.ReasonCode,
		"action_id":           action.ID,
		"effective":           action.Status == ActionApplied && (action.ReadbackVerified || action.Type != ActionSetScene && action.Type != ActionSetRoleState && action.Type != ActionFocusMemory),
		"readback_verified":   action.ReadbackVerified,
		"effective_from_turn": action.EffectiveFromTurn,
		"expires_after_turn":  action.ExpiresAfterTurn,
	}
	if err != nil {
		data["error"] = safeActionError(err)
	}
	emitRole(hooks.OnRoleEvent, RoleEvent{
		Type: "director_review", RoleID: d.ID, RoleInstanceID: inst.ID,
		RoleSessionID: session.ID, WorldlineID: action.WorldlineID, Data: data,
	})
	if action.ID == "" || action.Type == ActionNoChange {
		return
	}
	emitRole(hooks.OnRoleEvent, RoleEvent{
		Type: "director_action", RoleID: d.ID, RoleInstanceID: inst.ID,
		RoleSessionID: session.ID, WorldlineID: action.WorldlineID,
		Data: map[string]any{
			"action_id": action.ID, "action": action.Type, "status": action.Status,
			"reason_code": action.ReasonCode, "before": action.Before, "after": action.After,
			"readback_verified": action.ReadbackVerified, "effective_from_turn": action.EffectiveFromTurn,
			"expires_after_turn": action.ExpiresAfterTurn,
		},
	})
	if next := action.After["worldline_id"]; next != "" && next != session.WorldlineID {
		emitRole(hooks.OnRoleEvent, RoleEvent{
			Type: "worldline_forked", RoleID: d.ID, RoleInstanceID: inst.ID,
			RoleSessionID: session.ID, WorldlineID: next,
			Data: map[string]any{"action_id": action.ID, "from": session.WorldlineID, "to": next},
		})
	}
	if action.Type == ActionPauseRole && action.Status == ActionApplied {
		emitRole(hooks.OnRoleEvent, RoleEvent{
			Type: "role_paused", RoleID: d.ID, RoleInstanceID: inst.ID,
			RoleSessionID: session.ID, WorldlineID: action.WorldlineID,
			Data: map[string]any{"action_id": action.ID},
		})
	}
	if action.Type == ActionExitRole && action.Status == ActionApplied {
		emitRole(hooks.OnRoleEvent, RoleEvent{
			Type: "role_exited", RoleID: d.ID, RoleInstanceID: inst.ID,
			RoleSessionID: session.ID, WorldlineID: action.WorldlineID,
			Data: map[string]any{"action_id": action.ID},
		})
	}
}
