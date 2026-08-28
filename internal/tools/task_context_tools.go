package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/observe"
)

// TaskContextFocusController is session-only; implementations must not persist to LTM.
type TaskContextFocusController interface {
	Current(sessionID string) (workspaceID, intentID string)
	Apply(sessionID string, event observe.TaskContextFocusTrace)
}

func appendTaskContextFocusTool(toolsOut []tool.BaseTool, deps Deps) ([]tool.BaseTool, error) {
	if deps.TaskContextFocus == nil || !strings.EqualFold(strings.TrimSpace(deps.RecallMode), "agent") {
		return toolsOut, nil
	}
	report, err := utils.InferTool(
		"report_context_focus",
		"声明任务焦点或歧义；新 ID 须先 read。continue 也用于明确继续命名的既有任务及主项目附带辅助核对；switch 仅用于已有焦点被不同 ID 替换；check/compare 不建立连续焦点。不写 LTM。",
		func(ctx context.Context, in reportContextFocusInput) (string, error) {
			currentWorkspaceID, currentIntentID := deps.TaskContextFocus.Current(deps.SessionID)
			event, err := observe.RecordTaskContextFocus(ctx, observe.TaskContextFocusTrace{
				Action: strings.TrimSpace(in.Action), Certainty: strings.TrimSpace(in.Certainty),
				WorkspaceID: strings.TrimSpace(in.WorkspaceID), IntentID: strings.TrimSpace(in.IntentID),
				NeedsUserConfirmation: in.NeedsUserConfirmation,
			}, currentWorkspaceID, currentIntentID)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			deps.TaskContextFocus.Apply(deps.SessionID, event)
			workspaceID, intentID := deps.TaskContextFocus.Current(deps.SessionID)
			out, err := json.Marshal(map[string]any{
				"ok": true, "focus": event,
				"session_focus": map[string]string{"workspace_id": workspaceID, "intent_id": intentID},
			})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	return append(toolsOut, report), nil
}

type reportContextFocusInput struct {
	Action                string `json:"action" jsonschema:"description=continue|switch|check|compare|clarify|clear；continue=延续同一任务、明确命名的既有任务或主任务附带辅助核对；switch=用不同 ID 替换已有会话焦点；check/compare=一次性核对/比较且不建立连续焦点"`
	Certainty             string `json:"certainty" jsonschema:"description=clear|ambiguous"`
	WorkspaceID           string `json:"workspace_id,omitempty" jsonschema:"description=实际选中的 Workspace id；选择新 id 前必须 read_workspace"`
	IntentID              string `json:"intent_id,omitempty" jsonschema:"description=实际选中的 Intent id；选择新 id 前必须 read_intent"`
	NeedsUserConfirmation bool   `json:"needs_user_confirmation,omitempty" jsonschema:"description=只有 clarify/ambiguous 时为 true"`
}
