package theater

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func InitializationTools(architect *CharacterArchitect) ([]tool.BaseTool, error) {
	if architect == nil || architect.Store == nil {
		return nil, fmt.Errorf("character architect required")
	}
	start, err := utils.InferTool("start_role_initialization", "为一个人物或专业角色建立独立培养任务。先生成研究计划，必须等用户确认后才会联网研究；私人角色必须先明确同意将资料发送给模型。", func(ctx context.Context, in struct {
		DisplayName         string       `json:"display_name"`
		Kind                RoleKind     `json:"kind"`
		SubjectClass        SubjectClass `json:"subject_class"`
		Description         string       `json:"description,omitempty"`
		Objective           string       `json:"objective"`
		TargetPeriod        string       `json:"target_period,omitempty"`
		KnowledgeCutoff     string       `json:"knowledge_cutoff,omitempty"`
		VariantOfRoleID     string       `json:"variant_of_role_id,omitempty"`
		PrivateModelConsent bool         `json:"private_model_consent,omitempty"`
	}) (string, error) {
		run, role, startErr := architect.Start(ctx, StartRoleInitializationInput{DisplayName: in.DisplayName, Kind: in.Kind, SubjectClass: in.SubjectClass, Description: in.Description, Objective: in.Objective, TargetPeriod: in.TargetPeriod, KnowledgeCutoff: in.KnowledgeCutoff, VariantOfRoleID: in.VariantOfRoleID, PrivateModelConsent: in.PrivateModelConsent})
		return toolResult(map[string]any{"run": run, "role": role}, startErr)
	})
	if err != nil {
		return nil, err
	}
	inspect, err := utils.InferTool("inspect_role_initialization", "查看角色培养的研究计划、预算、来源采用状态、覆盖矩阵、冲突、Blueprint 和 Critic 结果。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		run, getErr := architect.Store.GetInitialization(ctx, strings.TrimSpace(in.ID))
		if getErr != nil {
			return toolResult(nil, getErr)
		}
		payload := map[string]any{"run": run}
		if run.BlueprintID != "" {
			if value, e := architect.Store.GetBlueprint(ctx, run.BlueprintID); e == nil {
				payload["blueprint"] = value
			}
		}
		if run.CritiqueID != "" {
			if value, e := architect.Store.GetCritique(ctx, run.CritiqueID); e == nil {
				payload["critique"] = value
			}
		}
		return toolResult(payload, nil)
	})
	if err != nil {
		return nil, err
	}
	approvePlan, err := utils.InferTool("approve_role_research_plan", "仅在用户明确确认研究计划后调用。确认后培养任务会在后台自主研究，直到需要追加预算、修订或最终确认。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		run, approveErr := architect.Store.ApproveResearchPlan(ctx, strings.TrimSpace(in.ID))
		if approveErr == nil {
			architect.ContinueAsync(run.ID)
		}
		return toolResult(map[string]any{"run": run, "started": approveErr == nil}, approveErr)
	})
	if err != nil {
		return nil, err
	}
	grant, err := utils.InferTool("grant_role_research_budget", "用户明确同意后，为培养任务增加 12 次远程操作额度，并继续研究；全局每日 40 次上限仍生效。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		run, grantErr := architect.Store.GrantInitializationBudget(ctx, strings.TrimSpace(in.ID))
		if grantErr == nil {
			architect.ContinueAsync(run.ID)
		}
		return toolResult(map[string]any{"run": run}, grantErr)
	})
	if err != nil {
		return nil, err
	}
	pause, err := utils.InferTool("pause_role_initialization", "暂停并保存角色培养 checkpoint。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		run, e := architect.Store.PauseInitialization(ctx, strings.TrimSpace(in.ID))
		return toolResult(map[string]any{"run": run}, e)
	})
	if err != nil {
		return nil, err
	}
	resume, err := utils.InferTool("resume_role_initialization", "从 checkpoint 恢复角色培养。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		run, e := architect.Store.ResumeInitialization(ctx, strings.TrimSpace(in.ID))
		if e == nil {
			architect.ContinueAsync(run.ID)
		}
		return toolResult(map[string]any{"run": run}, e)
	})
	if err != nil {
		return nil, err
	}
	retry, err := utils.InferTool("retry_role_initialization", "仅在角色培养失败后重试失败阶段；已读取证据和预算会保留，后台会从安全 checkpoint 继续。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		run, e := architect.Retry(ctx, strings.TrimSpace(in.ID))
		return toolResult(map[string]any{"run": run, "started": e == nil}, e)
	})
	if err != nil {
		return nil, err
	}
	cancel, err := utils.InferTool("cancel_role_initialization", "取消角色培养；不会删除已经保存的来源和审计记录。", func(ctx context.Context, in struct {
		ID string `json:"id"`
	}) (string, error) {
		run, e := architect.Store.CancelInitialization(ctx, strings.TrimSpace(in.ID))
		return toolResult(map[string]any{"run": run}, e)
	})
	if err != nil {
		return nil, err
	}
	approveFinal, err := utils.InferTool("approve_role_blueprint", "仅在用户查看 Blueprint 与 Critic 后明确确认上架时调用。普通警告必须填写接受理由；真实性硬错误不能绕过。", func(ctx context.Context, in struct {
		ID                      string `json:"id"`
		WarningAcceptanceReason string `json:"warning_acceptance_reason,omitempty"`
	}) (string, error) {
		run, role, e := architect.ApproveFinal(ctx, strings.TrimSpace(in.ID), in.WarningAcceptanceReason)
		return toolResult(map[string]any{"run": run, "role": role}, e)
	})
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{start, inspect, approvePlan, grant, pause, resume, retry, cancel, approveFinal}, nil
}

func toolResult(payload map[string]any, err error) (string, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	if err != nil {
		payload["ok"], payload["error"] = false, err.Error()
	} else {
		payload["ok"] = true
	}
	value, marshalErr := json.Marshal(payload)
	return string(value), marshalErr
}
