package theater

import (
	"log"
	"os"
	"strings"
)

// InitializationMode controls whether Character Architect may apply a
// validated blueprint to the formal RoleDefinition.
type InitializationMode string

const (
	InitModeOff     InitializationMode = "off"
	InitModeObserve InitializationMode = "observe"
	InitModeAgent   InitializationMode = "agent"
)

func ParseInitializationMode(raw string) InitializationMode {
	switch InitializationMode(strings.ToLower(strings.TrimSpace(raw))) {
	case InitModeObserve:
		return InitModeObserve
	case InitModeAgent:
		return InitModeAgent
	default:
		return InitModeOff
	}
}

func InitializationModeFromEnv() InitializationMode {
	raw := strings.TrimSpace(os.Getenv("ROLE_INIT_MODE"))
	mode := ParseInitializationMode(raw)
	if raw != "" && !strings.EqualFold(raw, string(mode)) {
		log.Printf("invalid ROLE_INIT_MODE %q; fallback off", raw)
	}
	return mode
}
