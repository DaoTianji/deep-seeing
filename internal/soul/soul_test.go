package soul_test

import (
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/soul"
)

func TestLoadSoulHasNoNamedPerson(t *testing.T) {
	path := filepath.Join("..", "..", "seed", "SOUL.md")
	text := soul.MustLoad(path)
	if text == "" {
		t.Fatal("empty soul")
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "mudnet") {
		t.Fatal("Soul must not name mudnet")
	}
	if strings.Contains(text, "导师") {
		t.Fatal("Soul must not prescribe 导师")
	}
	if !strings.Contains(text, "主体性") {
		t.Fatal("expected 主体性 section")
	}
}
