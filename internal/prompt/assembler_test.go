package prompt_test

import (
	"context"
	"strings"
	"testing"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/prompt"
)

func TestAssemblerIncludesSoulBodyOriginMemory(t *testing.T) {
	msgs, err := prompt.DefaultAssembler{}.BuildSystemMessages(context.Background(), prompt.AssembleInput{
		Scope:         identity.LocalCLI(),
		Soul:          "soul body",
		OriginContext: "origin letter",
		BondNorm:      "对该人常模尚薄，避免臆测人格；优先询问与观察。",
		MemoryRecall:  prompt.FormatMemoryRecall([]string{"[event/ep] 叫我安"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len=%d", len(msgs))
	}
	c := msgs[0].Content
	for _, want := range []string{"soul body", "origin letter", "叫我安", "Soul", "Origin Introduction", "Body / Capabilities", "list_capabilities", "Bond / Person norm", "常模尚薄"} {
		if !strings.Contains(c, want) {
			t.Fatalf("missing %q in %s", want, c)
		}
	}
}

func TestAssemblerOmitsEmptyOrigin(t *testing.T) {
	msgs, err := prompt.DefaultAssembler{}.BuildSystemMessages(context.Background(), prompt.AssembleInput{
		Soul: "soul",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msgs[0].Content, "Origin Introduction") {
		t.Fatal("origin section should be absent")
	}
}
