package runtime

import (
	"log"
	"os"
	"strings"
)

// RecallMode controls who decides whether long-term memory enters a turn.
type RecallMode string

const (
	RecallModeLegacy RecallMode = "legacy"
	RecallModeAgent  RecallMode = "agent"
)

// ParseRecallMode returns legacy for empty or unsupported values.
func ParseRecallMode(raw string) (RecallMode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(RecallModeLegacy):
		return RecallModeLegacy, true
	case string(RecallModeAgent):
		return RecallModeAgent, true
	default:
		return RecallModeLegacy, false
	}
}

// RecallModeFromEnv reads RECALL_MODE and logs invalid values before falling back.
func RecallModeFromEnv() RecallMode {
	raw := os.Getenv("RECALL_MODE")
	mode, valid := ParseRecallMode(raw)
	if !valid {
		log.Printf("invalid RECALL_MODE %q; using %s", strings.TrimSpace(raw), mode)
	}
	return mode
}
