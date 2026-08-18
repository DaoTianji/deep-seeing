package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"deep-seeing/internal/intent"
)

func appendIntentTools(toolsOut []tool.BaseTool, deps Deps, agentID string) ([]tool.BaseTool, error) {
	if deps.Intents == nil {
		return toolsOut, nil
	}
	store := deps.Intents

	listInt, err := utils.InferTool(
		"list_intents",
		"列出活跃 Intent（留给未来自己的待办/周期唤醒）。",
		func(ctx context.Context, in listIntentsInput) (string, error) {
			limit := in.Limit
			if limit <= 0 {
				limit = 30
			}
			items, err := store.ListActive(ctx, agentID, limit)
			if err != nil {
				return "", err
			}
			out, err := json.Marshal(map[string]any{"ok": true, "intents": items})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, listInt)

	readInt, err := utils.InferTool(
		"read_intent",
		"读取一条 Intent 与近期 wake 历史。",
		func(ctx context.Context, in readIntentInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			if id == "" {
				return `{"ok":false,"error":"id 不能为空"}`, nil
			}
			it, err := store.Get(ctx, id)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			wakes, _ := store.ListWakeJobs(ctx, id, 10)
			out, err := json.Marshal(map[string]any{"ok": true, "intent": it, "wake_jobs": wakes})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, readInt)

	createInt, err := utils.InferTool(
		"create_intent",
		"给未来的自己留下 Intent。默认不主动联系人（allow_outbound=false）。周期 Intent 需 interval。",
		func(ctx context.Context, in createIntentInput) (string, error) {
			title := strings.TrimSpace(in.Title)
			if title == "" {
				return `{"ok":false,"error":"title 不能为空"}`, nil
			}
			due, err := parseDue(in.DueAt)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			kind := intent.NormalizeKind(in.Kind)
			var interval time.Duration
			if in.IntervalHours > 0 {
				interval = time.Duration(in.IntervalHours) * time.Hour
			} else if in.IntervalDays > 0 {
				interval = time.Duration(in.IntervalDays) * 24 * time.Hour
			}
			it, err := store.Create(ctx, intent.CreateInput{
				AgentID: agentID, Kind: kind, Title: title, Body: strings.TrimSpace(in.Body),
				Trigger: intent.NormalizeTrigger(in.Trigger), DueAt: due, Interval: interval,
				StaleAfterDays: in.StaleAfterDays, AllowOutbound: in.AllowOutbound,
			})
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "intent": it})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	toolsOut = append(toolsOut, createInt)

	cancelInt, err := utils.InferTool(
		"cancel_intent",
		"取消一条尚未完成的 Intent。",
		func(ctx context.Context, in cancelIntentInput) (string, error) {
			id := strings.TrimSpace(in.ID)
			if id == "" {
				return `{"ok":false,"error":"id 不能为空"}`, nil
			}
			it, err := store.SetStatus(ctx, id, intent.StatusCancelled)
			if err != nil {
				out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
				return string(out), nil
			}
			out, err := json.Marshal(map[string]any{"ok": true, "intent": it})
			return string(out), err
		},
	)
	if err != nil {
		return nil, err
	}
	return append(toolsOut, cancelInt), nil
}

type listIntentsInput struct {
	Limit int `json:"limit,omitempty"`
}

type readIntentInput struct {
	ID string `json:"id"`
}

type createIntentInput struct {
	Kind           string `json:"kind,omitempty" jsonschema:"description=one_shot|recurring；缺省 recurring"`
	Title          string `json:"title"`
	Body           string `json:"body,omitempty"`
	DueAt          string `json:"due_at" jsonschema:"description=RFC3339 到期时间"`
	IntervalHours  int    `json:"interval_hours,omitempty" jsonschema:"description=周期小时数（recurring）"`
	IntervalDays   int    `json:"interval_days,omitempty" jsonschema:"description=周期天数（recurring）"`
	Trigger        string `json:"trigger,omitempty" jsonschema:"description=SELF_SCHEDULED|TASK_COMPLETED|MAINTENANCE"`
	StaleAfterDays int    `json:"stale_after_days,omitempty"`
	AllowOutbound  bool   `json:"allow_outbound,omitempty" jsonschema:"description=默认 false；是否允许主动联系人"`
}

type cancelIntentInput struct {
	ID string `json:"id"`
}

func parseDue(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errDueRequired
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errDueFormat
}

type simpleError string

func (e simpleError) Error() string { return string(e) }

const errDueRequired simpleError = "due_at required (RFC3339)"
const errDueFormat simpleError = "due_at must be RFC3339 or YYYY-MM-DD"
