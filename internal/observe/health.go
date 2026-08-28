package observe

import (
	"strings"

	"deep-seeing/internal/contextsource"
)

type TurnHealthStatus string

const (
	TurnHealthy  TurnHealthStatus = "healthy"
	TurnDegraded TurnHealthStatus = "degraded"
	TurnFailed   TurnHealthStatus = "failed"
)

type TurnIssueTrace struct {
	Code        string               `json:"code"`
	Component   string               `json:"component"`
	Source      contextsource.Source `json:"source,omitempty"`
	Recoverable bool                 `json:"recoverable"`
}

type TurnHealthTrace struct {
	Status TurnHealthStatus `json:"status"`
	Issues []TurnIssueTrace `json:"issues,omitempty"`
}

func SummarizeTurnHealth(trace TurnTrace) TurnHealthTrace {
	health := TurnHealthTrace{Status: TurnHealthy}
	seen := map[string]bool{}
	add := func(issue TurnIssueTrace) {
		key := issue.Code + "\x00" + issue.Component + "\x00" + string(issue.Source)
		if issue.Code == "" || seen[key] {
			return
		}
		seen[key] = true
		health.Issues = append(health.Issues, issue)
	}
	for _, source := range trace.ContextSources {
		switch strings.ToLower(strings.TrimSpace(source.State)) {
		case "unavailable":
			add(TurnIssueTrace{Code: "source_unavailable", Component: "context_source", Source: source.Source, Recoverable: true})
		case "error":
			add(TurnIssueTrace{Code: "source_error", Component: "context_source", Source: source.Source, Recoverable: true})
		}
		if strings.TrimSpace(source.Error) != "" {
			add(TurnIssueTrace{Code: "source_error", Component: "context_source", Source: source.Source, Recoverable: true})
		}
	}
	for _, candidate := range trace.ContextCandidates {
		if strings.TrimSpace(candidate.Error) != "" && candidate.Source != contextsource.Episode {
			add(TurnIssueTrace{Code: "search_error", Component: "context_candidate", Source: candidate.Source, Recoverable: true})
		}
	}
	for _, search := range trace.RecallSearches {
		if strings.TrimSpace(search.Error) != "" {
			add(TurnIssueTrace{Code: "search_error", Component: "search_episodes", Source: contextsource.Episode, Recoverable: true})
		}
	}
	for _, read := range trace.RecallReads {
		if strings.TrimSpace(read.Error) != "" {
			add(TurnIssueTrace{Code: "read_error", Component: "read_episode", Source: contextsource.Episode, Recoverable: true})
		}
	}
	for _, read := range trace.ContextReads {
		if read.Source == contextsource.Episode {
			continue
		}
		if strings.TrimSpace(read.Error) != "" || !read.OK {
			add(TurnIssueTrace{Code: "read_error", Component: "context_read", Source: read.Source, Recoverable: true})
		}
	}
	for _, expansion := range trace.ContextExpands {
		if strings.TrimSpace(expansion.Error) != "" && expansion.Operation == "list" {
			add(TurnIssueTrace{Code: "search_error", Component: "task_context_list", Source: contextsource.Source(expansion.Source), Recoverable: true})
		}
	}
	if trace.RecallMode == "agent" && trace.Attention == nil {
		add(TurnIssueTrace{Code: "attention_unavailable", Component: "attention_workspace", Recoverable: true})
	}
	for _, message := range trace.Errors {
		add(classifyTurnError(message))
	}
	if len(health.Issues) > 0 {
		health.Status = TurnDegraded
	}
	if strings.TrimSpace(trace.AnswerPreview) == "" && len(trace.Errors) > 0 {
		health.Status = TurnFailed
		for i := range health.Issues {
			if health.Issues[i].Component == "agent_stream" {
				health.Issues[i].Recoverable = false
			}
		}
	}
	return health
}

func classifyTurnError(message string) TurnIssueTrace {
	message = strings.ToLower(strings.TrimSpace(message))
	code := "stream_interrupted"
	switch {
	case strings.Contains(message, "max step") || strings.Contains(message, "exceeds max"):
		code = "step_limit"
	case strings.Contains(message, "deadline exceeded") || strings.Contains(message, "timeout") || strings.Contains(message, "timed out"):
		code = "timeout"
	}
	return TurnIssueTrace{Code: code, Component: "agent_stream", Recoverable: true}
}
