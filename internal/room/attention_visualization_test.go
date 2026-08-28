package room

import (
	"strings"
	"testing"
)

func TestRoomWebHandlesAttentionWorkspaceLiveAndReplay(t *testing.T) {
	js, css := builtFrontendAssets(t)
	for _, want := range []string{"attention_snapshot", "attention_decision", "意识中心", "支撑区", "外围", "公开状态"} {
		if !strings.Contains(js, want) {
			t.Fatalf("room app missing attention contract %q", want)
		}
	}
	for _, want := range []string{".attention-stage", ".attention-zone.center", ".attention-zone.support", ".attention-zone.periphery"} {
		if !strings.Contains(css, want) {
			t.Fatalf("room styles missing attention tier %q", want)
		}
	}
}
