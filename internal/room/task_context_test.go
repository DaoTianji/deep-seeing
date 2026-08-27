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
		`trace.task_context`,
		`trace.task_context_expansions`,
		"处境展开",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("room app missing task-context contract %q", want)
		}
	}
}
