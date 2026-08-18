// Package origin loads Origin Context letters (first introductions), not Soul instincts.
package origin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"deep-seeing/internal/identity"
)

// DefaultDir holds committed origin letters.
const DefaultDir = "seed/origin"

// PersonKeyMudnet is the file stem for mudnet's introduction.
const PersonKeyMudnet = "mudnet"

// Letter is one person's origin introduction.
type Letter struct {
	PersonKey string // e.g. mudnet
	PersonID  string // e.g. user:mudnet
	Body      string
}

// RoleAtOrigin is the only weak relational hint allowed at boot (not TRUSTS=high).
const RoleAtOrigin = "friend / early companion"

// Load reads seed/origin/{key}.md (key without user: prefix).
func Load(dir, personKey string) (Letter, error) {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultDir
	}
	personKey = strings.TrimSpace(personKey)
	personKey = strings.TrimPrefix(personKey, "user:")
	if personKey == "" {
		return Letter{}, fmt.Errorf("origin: empty person key")
	}
	path := filepath.Join(dir, personKey+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return Letter{}, err
	}
	return Letter{
		PersonKey: personKey,
		PersonID:  "user:" + personKey,
		Body:      strings.TrimSpace(string(data)),
	}, nil
}

// LoadForScope loads the origin letter for the current human in scope.
func LoadForScope(dir string, scope identity.TenantScope) (Letter, error) {
	return Load(dir, scope.UserID)
}

// FormatForPrompt wraps a letter for system injection.
func FormatForPrompt(letter Letter) string {
	if strings.TrimSpace(letter.Body) == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("以下是你刚开始认识世界时收到的一封自我介绍（Origin Context）。\n")
	b.WriteString("它不是你的本能，也不是不可修改的关系定义；真正的信任与相处方式，应由之后共同经历慢慢形成。\n")
	b.WriteString("弱先验：对方在起源时的角色可理解为「")
	b.WriteString(RoleAtOrigin)
	b.WriteString("」。不要预设 trust=high。\n\n")
	b.WriteString(letter.Body)
	return b.String()
}
