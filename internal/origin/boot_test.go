package origin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/origin"
)

func TestOriginIntroductionOnce(t *testing.T) {
	dir := t.TempDir()
	scope := identity.LocalCLI()
	letter := origin.Letter{PersonKey: "mudnet", PersonID: "user:mudnet", Body: "你好，我是 mudnet。"}
	gate := origin.BootGate{StateDir: dir}

	text, first, err := origin.IntroductionForBoot(gate, scope, letter, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first || !strings.Contains(text, "mudnet") {
		t.Fatalf("first=%v text=%q", first, text)
	}
	text2, first2, err := origin.IntroductionForBoot(gate, scope, letter, false)
	if err != nil {
		t.Fatal(err)
	}
	if first2 || text2 != "" {
		t.Fatalf("second should be empty, first2=%v text=%q", first2, text2)
	}
	if _, err := os.Stat(filepath.Join(dir, "origin_presented", "mudnet.flag")); err != nil {
		t.Fatal(err)
	}
}
