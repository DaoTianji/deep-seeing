package memory

import (
	"log"
	"os"
	"strings"
)

// ReflectionMode controls how the T3 reflection pipeline affects durable state.
type ReflectionMode string

const (
	ReflectionModeLegacy  ReflectionMode = "legacy"
	ReflectionModeObserve ReflectionMode = "observe"
	ReflectionModeAgent   ReflectionMode = "agent"
)

// ParseReflectionMode keeps unsupported configuration on the legacy-safe path.
// The shipped default is agent after the T3 behavioural gate passed.
func ParseReflectionMode(raw string) (ReflectionMode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ReflectionModeAgent, true
	case string(ReflectionModeObserve):
		return ReflectionModeObserve, true
	case string(ReflectionModeLegacy):
		return ReflectionModeLegacy, true
	case string(ReflectionModeAgent):
		return ReflectionModeAgent, true
	default:
		return ReflectionModeLegacy, false
	}
}

func ReflectionModeFromEnv() ReflectionMode {
	raw := os.Getenv("REFLECTION_MODE")
	mode, valid := ParseReflectionMode(raw)
	if !valid {
		log.Printf("invalid REFLECTION_MODE %q; using %s", strings.TrimSpace(raw), mode)
	}
	return mode
}
