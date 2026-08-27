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
		"公开声明本轮任务处境结论。Workspace 与 Intent 同时影响回答时，一次调用同时填写 workspace_id 和 intent_id。continue 用于延续同一焦点，或用户明确要求继续一个已有任务且没有替换另一个焦点；switch 仅用于已有会话焦点被不同 ID 替换；check 只核对；compare 只用于条目之间的对照，不用于项目加辅助提醒。新焦点必须先读取；clarify 声明歧义并要求用户确认。不是长期记忆，不要在无关普通任务中调用。",
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
	Action                string `json:"action" jsonschema:"description=continue|switch|check|compare|clarify|clear；continue=延续同一或明确命名的既有任务；switch=用不同 ID 替换已有会话焦点；compare=比较条目本身"`
	Certainty             string `json:"certainty" jsonschema:"description=clear|ambiguous"`
	WorkspaceID           string `json:"workspace_id,omitempty" jsonschema:"description=实际选中的 Workspace id；选择新 id 前必须 read_workspace"`
	IntentID              string `json:"intent_id,omitempty" jsonschema:"description=实际选中的 Intent id；选择新 id 前必须 read_intent"`
	NeedsUserConfirmation bool   `json:"needs_user_confirmation,omitempty" jsonschema:"description=只有 clarify/ambiguous 时为 true"`
}
