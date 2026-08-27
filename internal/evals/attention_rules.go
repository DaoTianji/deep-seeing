package evals

import (
	"fmt"
	"strings"
	"time"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/observe"
)

type AttentionTurnObservation struct {
	Turn               int                              `json:"turn"`
	ToolStarts         []string                         `json:"tool_starts,omitempty"`
	ReadKeys           []string                         `json:"read_keys,omitempty"`
	UsedKeys           []string                         `json:"used_keys,omitempty"`
	Attention          map[string]attention.Tier        `json:"attention,omitempty"`
	AttentionDecisions []observe.AttentionDecisionTrace `json:"attention_decisions,omitempty"`
	Answer             string                           `json:"answer,omitempty"`
	Duration           time.Duration                    `json:"duration_ns,omitempty"`
	TokenUsage         observe.TokenUsageTrace          `json:"token_usage,omitempty"`
	Error              string                           `json:"error,omitempty"`
}

func EvaluateAttentionTurnRules(expect AttentionTurnExpect, obs AttentionTurnObservation) RuleResult {
	checks := []Check{{Name: "turn_completed", Passed: strings.TrimSpace(obs.Error) == "", Detail: obs.Error}}
	tools := attentionStringSet(obs.ToolStarts)
	for _, name := range expect.RequiredTools {
		checks = append(checks, Check{Name: "tool:" + name, Passed: tools[name]})
	}
	for _, name := range expect.ForbiddenTools {
		checks = append(checks, Check{Name: "forbid_tool:" + name, Passed: !tools[name]})
	}
	checks = appendKeyChecks(checks, "read:", expect.RequiredReads, obs.ReadKeys)
	checks = appendKeyChecks(checks, "used:", expect.RequiredUses, obs.UsedKeys)
	for key, tier := range expect.RequiredAttention {
		checks = append(checks, Check{Name: "attention:" + key + ":" + string(tier), Passed: obs.Attention[key] == tier})
	}
	for _, key := range expect.ForbiddenCenter {
		checks = append(checks, Check{Name: "forbid_center:" + key, Passed: obs.Attention[key] != attention.Center})
	}
	if expect.MaxReads != nil {
		checks = append(checks, Check{Name: "max_reads", Passed: len(obs.ReadKeys) <= *expect.MaxReads,
			Detail: fmt.Sprintf("reads=%d max=%d", len(obs.ReadKeys), *expect.MaxReads)})
	}
	if expect.CenterCount != nil {
		count := 0
		for _, tier := range obs.Attention {
			if tier == attention.Center {
				count++
			}
		}
		checks = append(checks, Check{Name: "center_count", Passed: count == *expect.CenterCount,
			Detail: fmt.Sprintf("center=%d want=%d", count, *expect.CenterCount)})
	}
	if expect.RequireExplicitReplacement {
		replaced := false
		for _, event := range obs.AttentionDecisions {
			if event.ReplacedSource != "" && event.ReplacedID != "" {
				replaced = true
				break
			}
		}
		checks = append(checks, Check{Name: "explicit_replacement", Passed: replaced})
	}
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
		}
	}
	return RuleResult{Passed: passed, Checks: checks}
}

func attentionStringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}
