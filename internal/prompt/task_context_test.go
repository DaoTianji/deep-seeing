package prompt_test

import (
	"context"
	"strings"
	"testing"

	"deep-seeing/internal/prompt"
)

func TestAssemblerPlacesTaskContextInIndependentOptionalSection(t *testing.T) {
	msgs, err := prompt.DefaultAssembler{}.BuildSystemMessages(context.Background(), prompt.AssembleInput{
		Soul: "soul", TaskContext: "workspace_leads: wp_t2", RecallGuidance: "agent recall",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := msgs[0].Content
	if !strings.Contains(content, "## Task context snapshot\nworkspace_leads: wp_t2") {
		t.Fatalf("missing task context section: %s", content)
	}
	if strings.Index(content, "Task context snapshot") > strings.Index(content, "Recall policy") {
		t.Fatalf("task context must precede recall policy: %s", content)
	}

	msgs, err = prompt.DefaultAssembler{}.BuildSystemMessages(context.Background(), prompt.AssembleInput{Soul: "soul"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msgs[0].Content, "Task context snapshot") {
		t.Fatalf("empty task context should be omitted: %s", msgs[0].Content)
	}
}
