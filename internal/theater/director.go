package theater

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

type DirectorCompleter interface {
	Complete(context.Context, string, string) (string, error)
}

type DirectorReviewInput struct {
	Definition  RoleDefinition
	Instance    RoleInstance
	Session     RoleSession
	UserText    string
	ActorAnswer string
	TurnID      string
}

type directorDecision struct {
	Action           ActionType
	ReasonCode       string
	Scene            string
	StateKey         string
	StateValue       string
	MemoryContent    string
	MemoryID         string
	Label            string
	TouchesCanonical bool
}

type DirectorReviewer struct {
	Mode     Mode
	Store    *Store
	Episodes *memory.EpisodeStore
	Chat     DirectorCompleter
	Scope    identity.TenantScope
	Model    string
}

const directorReviewSystem = "你是安，正在幕后审视角色刚刚完成的一轮台前对话。\n" +
	"你的首选是 no_change；只有连续性、场景、安全、记忆或用户明确要求确实需要时才干预。\n" +
	"只返回一个 JSON 对象，不要解释。字段为 action, reason_code, scene, state_key, state_value, memory_content, memory_id, label, touches_canonical。\n" +
	"action 只能是 no_change, set_scene, set_role_state, focus_memory, append_simulated_memory, mask_simulated_memory, revise_role_model, fork_worldline, pause_role, exit_role。\n" +
	"reason_code 只能是 continuity, drift, scene, memory, canonical_change, safety, user_request, no_material_reason。\n" +
	"不得把幕后信息写进角色记忆，不得把生成内容称为史实；涉及 source-backed 事实或既有人生必须 touches_canonical=true。"

func (r *DirectorReviewer) ReviewAndApply(ctx context.Context, in DirectorReviewInput) (DirectorAction, error) {
	if r == nil || r.Store == nil {
		return DirectorAction{}, fmt.Errorf("director reviewer unavailable")
	}
	decision := directorDecision{Action: ActionNoChange, ReasonCode: "no_material_reason"}
	if r.Chat != nil {
		user := directorReviewInputText(in)
		raw, err := r.Chat.Complete(ctx, directorReviewSystem, user)
		if err == nil {
			if parsed, parseErr := parseDirectorDecision(raw); parseErr == nil {
				decision = parsed
			}
		}
	}
	action := DirectorAction{
		ID: "ract_" + compactUUID(), RoleID: in.Definition.ID,
		RoleInstanceID: in.Instance.ID, RoleSessionID: in.Session.ID,
		WorldlineID: in.Session.WorldlineID, TurnID: cleanText(in.TurnID),
		Type: decision.Action, ReasonCode: decision.ReasonCode,
		ExpectedVersion: in.Instance.Version, TouchesCanonical: decision.TouchesCanonical,
		Before: map[string]string{}, After: map[string]string{}, CreatedAt: time.Now().UTC(),
	}
	fillActionPayload(&action, decision, in)
	if r.Mode != ModeAgent {
		action.Status = ActionExpected
		return r.Store.RecordAction(ctx, action)
	}
	action.Status = ActionApplied
	applied, err := r.apply(ctx, action, decision, in)
	if err != nil {
		action.Status = ActionExpected
		action.ReasonCode = "safety"
		action.After = map[string]string{"error": safeActionError(err)}
		_, _ = r.Store.RecordAction(ctx, action)
		return action, err
	}
	return r.Store.RecordAction(ctx, applied)
}

func (r *DirectorReviewer) apply(ctx context.Context, action DirectorAction, d directorDecision, in DirectorReviewInput) (DirectorAction, error) {
	if action.TouchesCanonical && action.Type != ActionForkWorldline {
		world, inst, err := r.Store.ForkWorldline(ctx, action.ID, nonempty(d.Label, "canonical alternative"))
		if err != nil {
			return action, err
		}
		action.WorldlineID = world.ID
		action.AppliedVersion = inst.Version
		action.Before["parent_worldline_id"] = world.ParentWorldlineID
		action.After["worldline_id"] = world.ID
		in.Instance, in.Session.WorldlineID = inst, world.ID
	}
	switch action.Type {
	case ActionNoChange:
		action.AppliedVersion = in.Instance.Version
	case ActionSetScene:
		before, inst, err := r.Store.SetScene(ctx, in.Session.ID, in.Instance.Version, d.Scene)
		if err != nil {
			return action, err
		}
		action.Before["scene"], action.After["scene"] = before, inst.Scene
		action.AppliedVersion = inst.Version
	case ActionSetRoleState, ActionFocusMemory:
		key := d.StateKey
		if action.Type == ActionFocusMemory {
			key = "focus_memory"
		}
		value := d.StateValue
		if action.Type == ActionFocusMemory && value == "" {
			value = d.MemoryID
		}
		before, inst, err := r.Store.SetInstanceState(ctx, in.Session.ID, in.Instance.Version, key, value)
		if err != nil {
			return action, err
		}
		action.Before["state_key"], action.Before["state_value"] = key, before
		action.After["state_key"], action.After["state_value"] = key, value
		action.AppliedVersion = inst.Version
	case ActionAppendSimulatedMemory:
		if cleanText(d.MemoryContent) == "" {
			return action, fmt.Errorf("memory_content required")
		}
		mode := memory.ExperienceSimulatedRoleplay
		class := string(MemorySimulated)
		if in.Definition.Kind == RoleProfessional {
			mode = memory.ExperienceDelegatedRole
			class = string(MemoryOperational)
		}
		ep, err := r.Episodes.WriteEpisode(ctx, r.Scope, memory.EpisodeWrite{
			Content: d.MemoryContent, ExperienceMode: mode, RoleID: in.Definition.ID,
			RoleInstanceID: in.Instance.ID, RoleSessionID: in.Session.ID,
			WorldlineID: in.Session.WorldlineID, RoleMemoryClass: class,
			SessionID: in.Session.ID, Metadata: map[string]string{"director_action_id": action.ID},
		})
		if err != nil {
			return action, err
		}
		action.After["memory_id"] = ep.ID
		action.AppliedVersion = in.Instance.Version
	case ActionMaskSimulatedMemory:
		ep, err := r.Episodes.Get(ctx, d.MemoryID)
		if err != nil {
			return action, err
		}
		if ep.RoleID != in.Definition.ID || ep.RoleInstanceID != in.Instance.ID {
			return action, fmt.Errorf("memory belongs to another role")
		}
		if ep.RoleMemoryClass == string(MemoryCanonical) {
			return action, fmt.Errorf("canonical memory cannot be masked")
		}
		before, world, err := r.Store.SetMemoryMasked(ctx, in.Session.ID, ep.ID, true)
		if err != nil {
			return action, err
		}
		action.Before["masked"], action.After["masked"] = fmt.Sprint(before), "true"
		action.After["memory_id"] = ep.ID
		action.AppliedVersion = world.Version
	case ActionReviseRoleModel:
		if cleanText(d.StateKey) == "" {
			return action, fmt.Errorf("state_key required")
		}
		world, err := r.Store.GetWorldline(ctx, in.Session.WorldlineID)
		if err != nil {
			return action, err
		}
		before, updated, err := r.Store.SetWorldlineState(ctx, in.Session.ID, world.Version, d.StateKey, d.StateValue)
		if err != nil {
			return action, err
		}
		action.Before["state_key"], action.Before["state_value"] = d.StateKey, before
		action.After["state_key"], action.After["state_value"] = d.StateKey, d.StateValue
		action.AppliedVersion = updated.Version
	case ActionForkWorldline:
		world, inst, err := r.Store.ForkWorldline(ctx, action.ID, nonempty(d.Label, "director branch"))
		if err != nil {
			return action, err
		}
		action.Before["worldline_id"] = world.ParentWorldlineID
		action.After["worldline_id"] = world.ID
		action.WorldlineID, action.AppliedVersion = world.ID, inst.Version
	case ActionPauseRole:
		session, err := r.Store.Pause(ctx, "director")
		if err != nil {
			return action, err
		}
		action.After["session_status"] = string(session.Status)
	case ActionExitRole:
		session, err := r.Store.Exit(ctx, "director", false)
		if err != nil {
			return action, err
		}
		action.After["session_status"] = string(session.Status)
	default:
		return action, fmt.Errorf("unsupported director action")
	}
	return action, nil
}

func (r *DirectorReviewer) Revert(ctx context.Context, sessionID, actionID string) (DirectorAction, error) {
	actions, err := r.Store.ListActions(ctx, sessionID)
	if err != nil {
		return DirectorAction{}, err
	}
	var target *DirectorAction
	for i := range actions {
		if actions[i].RevertsActionID == actionID {
			return DirectorAction{}, fmt.Errorf("director action already reverted")
		}
		if actions[i].ID == actionID {
			copy := actions[i]
			target = &copy
		}
	}
	if target == nil || target.Status != ActionApplied {
		return DirectorAction{}, fmt.Errorf("applied director action not found")
	}
	current, err := r.Store.GetInstance(ctx, target.RoleInstanceID)
	if err != nil {
		return DirectorAction{}, err
	}
	revert := DirectorAction{
		ID: "ract_" + compactUUID(), RoleID: target.RoleID, RoleInstanceID: target.RoleInstanceID,
		RoleSessionID: target.RoleSessionID, WorldlineID: target.WorldlineID,
		Type: target.Type, Status: ActionReverted, ReasonCode: "user_request",
		RevertsActionID: target.ID, Before: cloneMap(target.After), After: cloneMap(target.Before),
		CreatedAt: time.Now().UTC(),
	}
	switch target.Type {
	case ActionSetScene:
		if current.Version != target.AppliedVersion {
			return DirectorAction{}, fmt.Errorf("role changed after director action")
		}
		_, updated, err := r.Store.SetScene(ctx, sessionID, current.Version, target.Before["scene"])
		if err != nil {
			return DirectorAction{}, err
		}
		revert.AppliedVersion = updated.Version
	case ActionSetRoleState, ActionFocusMemory:
		if current.Version != target.AppliedVersion {
			return DirectorAction{}, fmt.Errorf("role changed after director action")
		}
		_, updated, err := r.Store.SetInstanceState(ctx, sessionID, current.Version, target.Before["state_key"], target.Before["state_value"])
		if err != nil {
			return DirectorAction{}, err
		}
		revert.AppliedVersion = updated.Version
	case ActionMaskSimulatedMemory:
		worldBefore, getErr := r.Store.GetWorldline(ctx, target.WorldlineID)
		if getErr != nil {
			return DirectorAction{}, getErr
		}
		if worldBefore.Version != target.AppliedVersion {
			return DirectorAction{}, fmt.Errorf("worldline changed after director action")
		}
		_, world, err := r.Store.SetMemoryMasked(ctx, sessionID, target.After["memory_id"], false)
		if err != nil {
			return DirectorAction{}, err
		}
		revert.AppliedVersion = world.Version
	case ActionAppendSimulatedMemory:
		ep, err := r.Episodes.SetStatus(ctx, target.After["memory_id"], memory.EpisodeArchived, "reverted_director_action")
		if err != nil {
			return DirectorAction{}, err
		}
		revert.After["memory_status"] = string(ep.Status)
	case ActionReviseRoleModel:
		world, err := r.Store.GetWorldline(ctx, target.WorldlineID)
		if err != nil {
			return DirectorAction{}, err
		}
		if world.Version != target.AppliedVersion {
			return DirectorAction{}, fmt.Errorf("worldline changed after director action")
		}
		_, updated, err := r.Store.SetWorldlineState(ctx, sessionID, world.Version, target.Before["state_key"], target.Before["state_value"])
		if err != nil {
			return DirectorAction{}, err
		}
		revert.AppliedVersion = updated.Version
		if parent := target.Before["parent_worldline_id"]; parent != "" {
			inst, _, switchErr := r.Store.SwitchWorldline(ctx, sessionID, parent)
			if switchErr != nil {
				return DirectorAction{}, switchErr
			}
			revert.AppliedVersion = inst.Version
		}
	case ActionForkWorldline:
		branch, getErr := r.Store.GetWorldline(ctx, target.After["worldline_id"])
		if getErr != nil {
			return DirectorAction{}, getErr
		}
		if current.CurrentWorldlineID != branch.ID || branch.Version != 1 {
			return DirectorAction{}, fmt.Errorf("worldline changed after director action")
		}
		inst, _, err := r.Store.SwitchWorldline(ctx, sessionID, target.Before["worldline_id"])
		if err != nil {
			return DirectorAction{}, err
		}
		revert.AppliedVersion = inst.Version
	case ActionPauseRole:
		session, err := r.Store.Resume(ctx)
		if err != nil {
			return DirectorAction{}, err
		}
		revert.After["session_status"] = string(session.Status)
	case ActionNoChange:
	default:
		return DirectorAction{}, fmt.Errorf("director action cannot be safely reverted")
	}
	return r.Store.RecordAction(ctx, revert)
}

func directorReviewInputText(in DirectorReviewInput) string {
	return fmt.Sprintf("角色：%s\n类型：%s/%s\n时间边界：%s\n当前场景：%s\n当前状态：%v\n用户台前输入：%s\n角色回答：%s",
		in.Definition.DisplayName, in.Definition.Kind, in.Definition.SubjectClass,
		in.Definition.KnowledgeCutoff, in.Instance.Scene, in.Instance.State,
		cleanText(in.UserText), cleanText(in.ActorAnswer))
}

func parseDirectorDecision(raw string) (directorDecision, error) {
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return directorDecision{}, fmt.Errorf("director decision JSON missing")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw[start:end+1]), &m); err != nil {
		return directorDecision{}, err
	}
	d := directorDecision{
		Action: ActionType(mapString(m, "action")), ReasonCode: mapString(m, "reason_code"),
		Scene: mapString(m, "scene"), StateKey: mapString(m, "state_key"),
		StateValue: mapString(m, "state_value"), MemoryContent: mapString(m, "memory_content"),
		MemoryID: mapString(m, "memory_id"), Label: mapString(m, "label"),
		TouchesCanonical: mapBool(m, "touches_canonical"),
	}
	if !validActionType(d.Action) {
		return directorDecision{}, fmt.Errorf("invalid director action")
	}
	if !validReasonCode(d.ReasonCode) {
		return directorDecision{}, fmt.Errorf("invalid director reason code")
	}
	return d, nil
}

func fillActionPayload(a *DirectorAction, d directorDecision, in DirectorReviewInput) {
	switch d.Action {
	case ActionSetScene:
		a.Before["scene"], a.After["scene"] = in.Instance.Scene, d.Scene
	case ActionSetRoleState, ActionFocusMemory:
		key := d.StateKey
		if d.Action == ActionFocusMemory {
			key = "focus_memory"
		}
		value := d.StateValue
		if d.Action == ActionFocusMemory && value == "" {
			value = d.MemoryID
		}
		a.Before["state_key"], a.Before["state_value"] = key, in.Instance.State[key]
		a.After["state_key"], a.After["state_value"] = key, value
	case ActionAppendSimulatedMemory:
		a.After["memory_preview"] = truncateActionText(d.MemoryContent, 120)
	case ActionMaskSimulatedMemory:
		a.After["memory_id"] = d.MemoryID
	case ActionReviseRoleModel:
		a.After["state_key"], a.After["state_value"] = d.StateKey, d.StateValue
	case ActionForkWorldline:
		a.Before["worldline_id"], a.After["label"] = in.Session.WorldlineID, d.Label
	}
}

func mapString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return cleanText(v)
}
func mapBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}
func validReasonCode(v string) bool {
	switch v {
	case "continuity", "drift", "scene", "memory", "canonical_change", "safety", "user_request", "no_material_reason":
		return true
	default:
		return false
	}
}
func nonempty(v, fallback string) string {
	if cleanText(v) == "" {
		return fallback
	}
	return cleanText(v)
}
func truncateActionText(v string, n int) string {
	runes := []rune(cleanText(v))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[:n]) + "…"
}
func safeActionError(err error) string {
	if err == nil {
		return ""
	}
	return truncateActionText(err.Error(), 160)
}
