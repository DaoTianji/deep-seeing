package theater

import (
	"context"
	"fmt"
	"strings"

	"deep-seeing/internal/runtime"
)

func (r *Router) handleRoleCommand(ctx context.Context, message string, hooks RouterHooks) (runtime.TurnResult, bool, error) {
	raw := strings.TrimSpace(message)
	if !strings.HasPrefix(raw, "/role") {
		return runtime.TurnResult{}, false, nil
	}
	fields := strings.Fields(raw)
	if len(fields) < 2 {
		return runtime.TurnResult{}, true, fmt.Errorf("role command required")
	}
	var answer string
	var event RoleEvent
	switch fields[1] {
	case "enter":
		if len(fields) < 3 {
			return runtime.TurnResult{}, true, fmt.Errorf("role id required")
		}
		d, inst, session, err := r.Store.Enter(ctx, r.Scope, fields[2])
		if err != nil {
			return runtime.TurnResult{}, true, err
		}
		answer = "已进入角色：" + d.DisplayName
		event = RoleEvent{Type: "role_entered", RoleID: d.ID, RoleInstanceID: inst.ID, RoleSessionID: session.ID, WorldlineID: session.WorldlineID, Data: map[string]any{"display_name": d.DisplayName}}
		_ = IndexRole(ctx, r.Graph, r.Scope, r.Store, d.ID)
	case "pause":
		session, err := r.Store.Pause(ctx, "user_command")
		if err != nil {
			return runtime.TurnResult{}, true, err
		}
		answer = "角色已暂停并保存。"
		event = RoleEvent{Type: "role_paused", RoleID: session.RoleID, RoleInstanceID: session.RoleInstanceID, RoleSessionID: session.ID, WorldlineID: session.WorldlineID}
		_ = IndexRole(ctx, r.Graph, r.Scope, r.Store, session.RoleID)
	case "resume":
		session, err := r.Store.Resume(ctx)
		if err != nil {
			return runtime.TurnResult{}, true, err
		}
		answer = "角色已恢复。"
		event = RoleEvent{Type: "role_state", RoleID: session.RoleID, RoleInstanceID: session.RoleInstanceID, RoleSessionID: session.ID, WorldlineID: session.WorldlineID, Data: map[string]any{"status": session.Status}}
		_ = IndexRole(ctx, r.Graph, r.Scope, r.Store, session.RoleID)
	case "exit":
		session, err := r.Store.Exit(ctx, "user_command", false)
		if err != nil {
			return runtime.TurnResult{}, true, err
		}
		answer = "已强制退场，角色会话已存档。现在由安与你说话。"
		event = RoleEvent{Type: "role_exited", RoleID: session.RoleID, RoleInstanceID: session.RoleInstanceID, RoleSessionID: session.ID, WorldlineID: session.WorldlineID}
		_ = IndexRole(ctx, r.Graph, r.Scope, r.Store, session.RoleID)
	case "fork":
		label := strings.TrimSpace(strings.TrimPrefix(raw, "/role fork"))
		world, inst, err := r.Store.ForkWorldline(ctx, "", label)
		if err != nil {
			return runtime.TurnResult{}, true, err
		}
		answer = "已创建并切换到新世界线：" + world.Label
		event = RoleEvent{Type: "worldline_forked", RoleID: world.RoleID, RoleInstanceID: inst.ID, WorldlineID: world.ID, Data: map[string]any{"from": world.ParentWorldlineID, "to": world.ID}}
		_ = IndexRole(ctx, r.Graph, r.Scope, r.Store, world.RoleID)
	default:
		return runtime.TurnResult{}, true, fmt.Errorf("unknown role command")
	}
	if hooks.Turn.WriteDelta != nil {
		hooks.Turn.WriteDelta(answer)
	}
	emitRole(hooks.OnRoleEvent, event)
	return runtime.TurnResult{TurnID: hooks.Turn.TurnID, Answer: answer}, true, nil
}
