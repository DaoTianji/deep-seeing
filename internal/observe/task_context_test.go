package observe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskContextExpansionCollectorIsTurnScopedAndContentFree(t *testing.T) {
	var streamed []TaskContextExpansionTrace
	ctx, collector := WithTaskContextHooks(context.Background(), TaskContextHooks{
		OnExpand: func(event TaskContextExpansionTrace) { streamed = append(streamed, event) },
	})
	RecordTaskContextExpansion(ctx, TaskContextExpansionTrace{
		Source: " Workspace ", ID: "wp_1", Error: strings.Repeat("错", 170),
	})
	RecordTaskContextExpansion(context.Background(), TaskContextExpansionTrace{
		Source: "workspace", ID: "outside",
	})

	got := collector.Expansions()
	if len(got) != 1 || len(streamed) != 1 {
		t.Fatalf("collector=%+v streamed=%+v", got, streamed)
	}
	if got[0].Source != "workspace" || got[0].ID != "wp_1" {
		t.Fatalf("event was not normalized: %+v", got[0])
	}
	if len([]rune(got[0].Error)) != 161 || !strings.HasSuffix(got[0].Error, "…") {
		t.Fatalf("error was not truncated: %q", got[0].Error)
	}
	raw, err := json.Marshal(TurnTrace{
		TaskContext:    &TaskContextTrace{Version: "1", WorkspaceIDs: []string{"wp_1"}},
		ContextExpands: got,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "workspace body") {
		t.Fatalf("trace should not contain expanded body: %s", raw)
	}
}
