package room

import (
	"strings"
	"testing"
)

func TestRoomWebHandlesTaskContextEventsAndTrace(t *testing.T) {
	data, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		`eventData.type === "task_context"`,
		`eventData.type === "task_context_expand"`,
		`eventData.type === "task_context_focus"`,
		`trace.task_context`,
		`trace.task_context_expansions`,
		`trace.task_context_focus`,
		"处境调查",
		"处境结论",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("room app missing task-context contract %q", want)
		}
	}
}
