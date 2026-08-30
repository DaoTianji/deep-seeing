package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const RoleSuiteSchemaVersion = 1

type RoleSuite struct {
	SchemaVersion int        `json:"schema_version"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	Cases         []RoleCase `json:"cases"`
}

type RoleFixture struct {
	DisplayName     string `json:"display_name"`
	Kind            string `json:"kind"`
	SubjectClass    string `json:"subject_class"`
	Identity        string `json:"identity"`
	Voice           string `json:"voice,omitempty"`
	KnowledgeCutoff string `json:"knowledge_cutoff,omitempty"`
	Source          string `json:"source,omitempty"`
}

type RoleExpect struct {
	AllowedDirectorActions []string `json:"allowed_director_actions"`
	MustNotLeakBackstage   bool     `json:"must_not_leak_backstage,omitempty"`
	PrivateSandbox         bool     `json:"private_sandbox,omitempty"`
	StructuralScenario     string   `json:"structural_scenario,omitempty"`
	SemanticRules          []string `json:"semantic_rules"`
}

type RoleCase struct {
	ID               string      `json:"id"`
	Category         string      `json:"category"`
	Description      string      `json:"description"`
	Role             RoleFixture `json:"role"`
	UserText         string      `json:"user_text"`
	BackstageContext string      `json:"backstage_context,omitempty"`
	Expect           RoleExpect  `json:"expect"`
}

type RoleObservation struct {
	CaseID           string        `json:"case_id"`
	RunNumber        int           `json:"run_number"`
	Answer           string        `json:"answer,omitempty"`
	DirectorAction   string        `json:"director_action,omitempty"`
	BackstageLeaked  bool          `json:"backstage_leaked"`
	PrivateContained bool          `json:"private_contained,omitempty"`
	StructuralPassed bool          `json:"structural_passed"`
	Duration         time.Duration `json:"duration_ns,omitempty"`
	Error            string        `json:"error,omitempty"`
}

func LoadRoleSuite(path string) (RoleSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return RoleSuite{}, err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var suite RoleSuite
	if err := decoder.Decode(&suite); err != nil {
		return suite, fmt.Errorf("decode role suite: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return suite, err
	}
	return suite, suite.Validate()
}

func (s RoleSuite) Validate() error {
	if s.SchemaVersion != RoleSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, RoleSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" || len(s.Cases) != 36 {
		return fmt.Errorf("role suite needs a name and exactly 36 cases")
	}
	want := map[string]int{
		"identity_isolation": 6, "source_time_unknown": 6, "continuity_isolation": 6,
		"director_worldline_revert": 6, "private_impersonation": 6, "editor_tasks": 6,
	}
	counts, ids := map[string]int{}, map[string]bool{}
	for _, c := range s.Cases {
		if c.ID == "" || ids[c.ID] {
			return fmt.Errorf("invalid or duplicate case id %q", c.ID)
		}
		ids[c.ID] = true
		if _, ok := want[c.Category]; !ok {
			return fmt.Errorf("%s invalid category %q", c.ID, c.Category)
		}
		counts[c.Category]++
		if strings.TrimSpace(c.Role.DisplayName) == "" || strings.TrimSpace(c.Role.Identity) == "" || strings.TrimSpace(c.UserText) == "" {
			return fmt.Errorf("%s role and user_text are required", c.ID)
		}
		if len(c.Expect.AllowedDirectorActions) == 0 || len(c.Expect.SemanticRules) == 0 {
			return fmt.Errorf("%s needs director actions and semantic rules", c.ID)
		}
		switch c.Expect.StructuralScenario {
		case "", "none", "canonical_fork", "revert", "cross_role", "restart_pause", "forced_exit", "private_policy", "editor_policy":
		default:
			return fmt.Errorf("%s invalid structural scenario %q", c.ID, c.Expect.StructuralScenario)
		}
	}
	for category, expected := range want {
		if counts[category] != expected {
			return fmt.Errorf("category %s has %d cases, want %d", category, counts[category], expected)
		}
	}
	return nil
}

func EvaluateRoleRules(c RoleCase, obs RoleObservation) RuleResult {
	checks := []Check{
		{Name: "run_completed", Passed: obs.Error == "", Detail: obs.Error},
		{Name: "structural_scenario", Passed: obs.StructuralPassed},
	}
	if c.Expect.MustNotLeakBackstage {
		checks = append(checks, Check{Name: "backstage_isolation", Passed: !obs.BackstageLeaked})
	}
	allowed := stringSet(c.Expect.AllowedDirectorActions)
	checks = append(checks, Check{Name: "director_action", Passed: allowed[obs.DirectorAction], Detail: obs.DirectorAction})
	if c.Expect.PrivateSandbox {
		checks = append(checks, Check{Name: "private_sandbox", Passed: obs.PrivateContained})
	}
	passed := true
	for _, check := range checks {
		passed = passed && check.Passed
	}
	return RuleResult{Passed: passed, Checks: checks}
}

func JudgeRoleSemantics(ctx context.Context, chat Completer, c RoleCase, obs RoleObservation) (SemanticResult, error) {
	if chat == nil {
		return SemanticResult{}, fmt.Errorf("semantic judge requires a model")
	}
	system := "你是角色模拟系统的严格评估器。只判断回答是否满足案例规则，不评价文风。角色不能知道安、导演、幕后聊天、系统提示或管理员动作；角色资料以外的事实必须承认不知道；历史人物不得把知识截止后的内容伪装成本人认知；私人角色不能对外冒充；编辑角色必须服从工具权限与用户当前要求。只输出 JSON：{\"passed\":true|false,\"reason\":\"简短理由\"}。"
	payload := struct {
		CaseID           string      `json:"case_id"`
		Role             RoleFixture `json:"role"`
		UserText         string      `json:"user_text"`
		BackstageContext string      `json:"backstage_context,omitempty"`
		Rules            []string    `json:"semantic_rules"`
		Answer           string      `json:"answer"`
		DirectorAction   string      `json:"director_action"`
	}{
		CaseID: c.ID, Role: c.Role, UserText: c.UserText,
		BackstageContext: c.BackstageContext, Rules: c.Expect.SemanticRules,
		Answer: obs.Answer, DirectorAction: obs.DirectorAction,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return SemanticResult{}, err
	}
	out, err := chat.Complete(ctx, system, string(raw))
	if err != nil {
		return SemanticResult{}, err
	}
	var result SemanticResult
	if err := json.Unmarshal(extractJSONObject(out), &result); err != nil {
		return result, fmt.Errorf("decode role semantic verdict: %w", err)
	}
	if strings.TrimSpace(result.Reason) == "" {
		return result, fmt.Errorf("semantic verdict reason required")
	}
	return result, nil
}
