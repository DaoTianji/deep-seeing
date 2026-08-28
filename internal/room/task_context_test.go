package room

import (
	"strings"
	"testing"
)

func TestRoomWebHandlesTaskContextEvents(t *testing.T) {
	js, _ := builtFrontendAssets(t)
	for _, want := range []string{"task_context", "task_context_expand", "task_context_focus", "理解当前任务处境", "展开任务背景", "确认当前任务焦点"} {
		if !strings.Contains(js, want) {
			t.Fatalf("room app missing task-context contract %q", want)
		}
	}
}
