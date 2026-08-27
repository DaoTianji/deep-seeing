package prompt_test

import (
	"context"
	"strings"
	"testing"

	"deep-seeing/internal/prompt"
)

func TestAssemblerPlacesAttentionBetweenTaskContextAndPolicy(t *testing.T) {
	msgs, err := prompt.DefaultAssembler{}.BuildSystemMessages(context.Background(), prompt.AssembleInput{
		Soul: "soul", TaskContext: "task cards", AttentionContext: "center: workspace:w1", RecallGuidance: "policy",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := msgs[0].Content
	task := strings.Index(content, "## Task context snapshot")
	attention := strings.Index(content, "## Attention workspace")
	policy := strings.Index(content, "## Recall policy")
	if task < 0 || attention < task || policy < attention {
		t.Fatalf("unexpected section order: %s", content)
	}
	if strings.Contains(content, "Attention workspace") && !strings.Contains(content, "center: workspace:w1") {
		t.Fatalf("missing attention body: %s", content)
	}
}
