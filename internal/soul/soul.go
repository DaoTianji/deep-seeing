package soul

import (
	_ "embed"
	"os"
	"strings"
)

//go:embed SOUL.md
var embeddedSOUL string

// DefaultSoulPath is the editable seed file in the repo.
const DefaultSoulPath = "seed/SOUL.md"

// Load reads Soul text from path; empty path uses DefaultSoulPath; missing file falls back to embed.
func Load(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultSoulPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return strings.TrimSpace(embeddedSOUL), nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// MustLoad is Load with embed fallback on any read error.
func MustLoad(path string) string {
	s, err := Load(path)
	if err != nil || s == "" {
		return strings.TrimSpace(embeddedSOUL)
	}
	return s
}
