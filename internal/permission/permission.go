package permission

// Level is the three-tier tool permission model (birth-gate).
type Level string

const (
	Observe  Level = "observe"  // search, read, recall
	Internal Level = "internal" // reversible / sandbox / memory writes
	External Level = "external" // irreversible real-world side effects — requires interrupt
)

// RequiresInterrupt reports whether human confirmation is mandatory before running.
func RequiresInterrupt(level Level) bool {
	return level == External
}

// Classify maps a tool name to its permission level.
func Classify(toolName string) Level {
	switch toolName {
	case "inspect_runtime", "list_capabilities", "tool_help", "get_time",
		"read_episode", "search_episodes", "recall_bond",
		"list_scene_norms", "read_scene_norm",
		"inspect_self", "trace_self_belief", "list_self_tensions",
		"list_workspace", "read_workspace",
		"list_intents", "read_intent",
		"list_sources", "read_source":
		return Observe
	case "write_episode", "archive_episode", "invalidate_episode",
		"propose_bond_update", "propose_self_update", "set_explicit_bond_fact",
		"append_bond_boundary", "set_bond_strategy_cache", "write_scene_norm",
		"write_workspace", "link_workspace_episode",
		"create_intent", "cancel_intent":
		return Internal
	case "search_web", "read_webpage", "web_search", "web_read",
		"send_message", "shell_host", "payment":
		return External
	default:
		// unknown tools default to external until explicitly classified
		return External
	}
}

// AllowAuto returns false when the runtime must interrupt before execution.
func AllowAuto(toolName string) bool {
	return !RequiresInterrupt(Classify(toolName))
}
