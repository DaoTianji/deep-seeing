package origin_test

import (
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/origin"
)

func TestLoadMudnetOrigin(t *testing.T) {
	dir := filepath.Join("..", "..", "seed", "origin")
	letter, err := origin.LoadForScope(dir, identity.LocalCLI())
	if err != nil {
		t.Fatal(err)
	}
	if letter.PersonID != "user:mudnet" {
		t.Fatalf("person=%s", letter.PersonID)
	}
	if !strings.Contains(letter.Body, "mudnet") {
		t.Fatal("expected self-intro")
	}
	prompt := origin.FormatForPrompt(letter)
	if !strings.Contains(prompt, "Origin Context") {
		t.Fatal("expected wrapper")
	}
	if !strings.Contains(prompt, origin.RoleAtOrigin) {
		t.Fatal("expected weak role prior")
	}
	if !strings.Contains(prompt, "不要预设 trust=high") {
		t.Fatal("expected anti trust=high prior in wrapper")
	}
}
