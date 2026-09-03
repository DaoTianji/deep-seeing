package theater

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/identity"
)

type DirectorToolDeps struct {
	Scope    identity.TenantScope
	Mode     Mode
	Store    *Store
	Reviewer *DirectorReviewer
}

func DirectorTools(deps DirectorToolDeps) ([]tool.BaseTool, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("role store required")
	}
	listTool, err := utils.InferTool("list_roles", "列出角色库；ready 角色可进入，draft/validating 尚未上架。", func(ctx context.Context, in struct {
		IncludeArchived bool `json:"include_archived,omitempty"`
	}) (string, error) {
		items, listErr := deps.Store.ListDefinitions(ctx, deps.Scope, in.IncludeArchived)
		if listErr != nil {
			return "", listErr
		}
		out, err := json.Marshal(map[string]any{"ok": true, "roles": items})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	inspectTool, err := utils.InferTool("inspect_role", "读取一个角色的定义、主张，以及当前活跃实例状态；这是幕后能力。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		d, getErr := deps.Store.GetDefinition(ctx, strings.TrimSpace(in.ID))
		if getErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": getErr.Error()})
			return string(out), nil
		}
		claims, _ := deps.Store.ListClaims(ctx, d.ID)
		sources, _ := deps.Store.ListSources(ctx, d.ID)
		payload := map[string]any{"ok": true, "role": d, "claims": claims, "sources": sources}
		if activeD, inst, session, activeErr := deps.Store.Active(ctx); activeErr == nil && activeD.ID == d.ID {
			payload["instance"], payload["session"] = inst, session
		}
		out, err := json.Marshal(payload)
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	readSourceTool, err := utils.InferTool("read_role_source", "在幕后读取角色来源正文。director 来源永远不会进入 Actor 编译或上下文。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		source, body, readErr := deps.Store.GetSource(ctx, strings.TrimSpace(in.ID))
		if readErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": readErr.Error()})
			return string(out), nil
		}
		text := string(body)
		if len([]rune(text)) > 20000 {
			text = string([]rune(text)[:20000]) + "…"
		}
		out, marshalErr := json.Marshal(map[string]any{"ok": true, "source": source, "content": text})
		return string(out), marshalErr
	})
	if err != nil {
		return nil, err
	}
	enterTool, err := utils.InferTool("enter_role", "进入一个已经验收并上架的角色；同一时刻只能有一个角色会话。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		if deps.Mode == ModeOff {
			return "{\"ok\":false,\"error\":\"ROLE_MODE is off\"}", nil
		}
		d, inst, session, enterErr := deps.Store.Enter(ctx, deps.Scope, strings.TrimSpace(in.ID))
		if enterErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": enterErr.Error()})
			return string(out), nil
		}
		out, err := json.Marshal(map[string]any{"ok": true, "role": d, "instance": inst, "session": session})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	pauseTool, err := utils.InferTool("pause_role", "强制暂停并存档当前角色状态；角色不能拒绝。", func(ctx context.Context, in struct {
		Reason string `json:"reason,omitempty"`
	}) (string, error) {
		if deps.Reviewer != nil {
			action, applyErr := deps.Reviewer.ApplyRequested(ctx, "", DirectorActionRequest{Action: ActionPauseRole, ReasonCode: "user_request"})
			if applyErr != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": applyErr.Error()})
				return string(out), nil
			}
			out, marshalErr := json.Marshal(map[string]any{"ok": true, "action": action})
			return string(out), marshalErr
		}
		session, pauseErr := deps.Store.Pause(ctx, in.Reason)
		if pauseErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": pauseErr.Error()})
			return string(out), nil
		}
		out, err := json.Marshal(map[string]any{"ok": true, "session": session})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	resumeTool, err := utils.InferTool("resume_role", "恢复当前已经暂停的角色会话。", func(ctx context.Context, _ struct{}) (string, error) {
		session, resumeErr := deps.Store.Resume(ctx)
		if resumeErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": resumeErr.Error()})
			return string(out), nil
		}
		out, err := json.Marshal(map[string]any{"ok": true, "session": session})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	exitTool, err := utils.InferTool("exit_role", "强制退出当前角色并存档本次 Session；角色实例保留供以后继续。", func(ctx context.Context, in struct {
		Reason string `json:"reason,omitempty"`
	}) (string, error) {
		if deps.Reviewer != nil {
			action, applyErr := deps.Reviewer.ApplyRequested(ctx, "", DirectorActionRequest{Action: ActionExitRole, ReasonCode: "user_request"})
			if applyErr != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": applyErr.Error()})
				return string(out), nil
			}
			out, marshalErr := json.Marshal(map[string]any{"ok": true, "action": action})
			return string(out), marshalErr
		}
		session, exitErr := deps.Store.Exit(ctx, in.Reason, false)
		if exitErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": exitErr.Error()})
			return string(out), nil
		}
		out, err := json.Marshal(map[string]any{"ok": true, "session": session})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	forkTool, err := utils.InferTool("fork_role_worldline", "从当前角色人生创建新世界线；原世界线保持不变。", func(ctx context.Context, in struct {
		Label string `json:"label,omitempty"`
	}) (string, error) {
		if deps.Reviewer != nil {
			action, applyErr := deps.Reviewer.ApplyRequested(ctx, "", DirectorActionRequest{Action: ActionForkWorldline, ReasonCode: "user_request", Label: in.Label})
			if applyErr != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": applyErr.Error()})
				return string(out), nil
			}
			out, marshalErr := json.Marshal(map[string]any{"ok": true, "action": action})
			return string(out), marshalErr
		}
		world, inst, forkErr := deps.Store.ForkWorldline(ctx, "", in.Label)
		if forkErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": forkErr.Error()})
			return string(out), nil
		}
		out, err := json.Marshal(map[string]any{"ok": true, "worldline": world, "instance": inst})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	interveneTool, err := utils.InferTool("apply_role_intervention", "在幕后对当前角色执行结构化干预。action 只能是 no_change、set_scene、set_role_state、focus_memory、append_simulated_memory、mask_simulated_memory、revise_role_model、fork_worldline、pause_role、exit_role；reason_code 只能是 continuity、drift、scene、memory、canonical_change、safety、user_request、no_material_reason。改变表演方式时使用 set_role_state，state_key 必须为 performance_style，state_value 写完整风格；可选 energy、stance、initiative、response_policy、intensity(0-10)、scope(next_turn|turns|session)、expires_after_turns。默认作用三轮。只有返回 effective=true、action.status=applied 且 readback_verified=true 才能声称已经生效；expected 只是观察建议，rejected 表示没有生效。", func(ctx context.Context, in DirectorActionRequest) (string, error) {
		if deps.Reviewer == nil {
			return "{\"ok\":false,\"error\":\"director reviewer unavailable\"}", nil
		}
		action, applyErr := deps.Reviewer.ApplyRequested(ctx, "", in)
		if applyErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "effective": false, "action": action, "error": applyErr.Error()})
			return string(out), nil
		}
		effective := action.Status == ActionApplied && action.ReadbackVerified
		if action.Type == ActionNoChange || action.Type == ActionAppendSimulatedMemory || action.Type == ActionMaskSimulatedMemory || action.Type == ActionForkWorldline || action.Type == ActionPauseRole || action.Type == ActionExitRole || action.Type == ActionReviseRoleModel {
			effective = action.Status == ActionApplied
		}
		out, marshalErr := json.Marshal(map[string]any{"ok": true, "effective": effective, "action": action})
		return string(out), marshalErr
	})
	if err != nil {
		return nil, err
	}
	transcriptTool, err := utils.InferTool("read_role_transcript", "在幕后读取当前角色的台前或幕后记录；正文不会提供给角色。", func(ctx context.Context, in struct {
		Channel string `json:"channel"`
		Limit   int    `json:"limit,omitempty"`
	}) (string, error) {
		_, _, session, activeErr := deps.Store.Active(ctx)
		if activeErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": activeErr.Error()})
			return string(out), nil
		}
		channel := Channel(strings.TrimSpace(in.Channel))
		items, readErr := deps.Store.ReadTranscript(ctx, session.ID, channel, in.Limit)
		if readErr != nil {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": readErr.Error()})
			return string(out), nil
		}
		out, err := json.Marshal(map[string]any{"ok": true, "channel": channel, "messages": items})
		return string(out), err
	})
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{listTool, inspectTool, readSourceTool, enterTool, pauseTool, resumeTool, exitTool, forkTool, interveneTool, transcriptTool}, nil
}
