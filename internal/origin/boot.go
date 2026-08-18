package origin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"deep-seeing/internal/identity"
)

// DefaultStateDir holds first-boot markers (Origin letter shown once).
const DefaultStateDir = "data/memory/state"

// BootGate tracks whether Origin Introduction was already presented.
type BootGate struct {
	StateDir string
}

func (g BootGate) stateDir() string {
	if strings.TrimSpace(g.StateDir) == "" {
		return DefaultStateDir
	}
	return g.StateDir
}

func (g BootGate) markerPath(scope identity.TenantScope) string {
	key := strings.TrimSpace(scope.UserID)
	if key == "" {
		key = "unknown"
	}
	return filepath.Join(g.stateDir(), "origin_presented", key+".flag")
}

// AlreadyPresented reports whether Origin Introduction was shown before.
func (g BootGate) AlreadyPresented(scope identity.TenantScope) bool {
	_, err := os.Stat(g.markerPath(scope))
	return err == nil
}

// MarkPresented records that Origin Introduction was injected once.
func (g BootGate) MarkPresented(scope identity.TenantScope) error {
	path := g.markerPath(scope)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("presented\n"), 0o644)
}

// IntroductionForBoot returns prompt text only on first boot.
// After first presentation, Origin letter stays on disk but is not re-injected every turn.
func IntroductionForBoot(gate BootGate, scope identity.TenantScope, letter Letter, force bool) (text string, first bool, err error) {
	if strings.TrimSpace(letter.Body) == "" {
		return "", false, nil
	}
	if !force && gate.AlreadyPresented(scope) {
		return "", false, nil
	}
	text = FormatForPrompt(letter)
	if text == "" {
		return "", false, nil
	}
	if err := gate.MarkPresented(scope); err != nil {
		return text, true, fmt.Errorf("mark origin presented: %w", err)
	}
	return text, true, nil
}
