package theater

import (
	"log"
	"os"
	"strings"
)

func ParseMode(raw string) Mode {
	switch Mode(strings.ToLower(strings.TrimSpace(raw))) {
	case ModeObserve:
		return ModeObserve
	case ModeAgent:
		return ModeAgent
	default:
		return ModeOff
	}
}

func ModeFromEnv() Mode {
	raw := strings.TrimSpace(os.Getenv("ROLE_MODE"))
	mode := ParseMode(raw)
	if raw != "" && !strings.EqualFold(raw, string(mode)) {
		log.Printf("invalid ROLE_MODE %q; fallback off", raw)
	}
	return mode
}
