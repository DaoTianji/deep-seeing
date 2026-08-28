package room

import (
	"strings"
	"testing"
)

func TestRoomWebHandlesUnifiedContextActivationAndReplay(t *testing.T) {
	js, css := builtFrontendAssets(t)
	for _, want := range []string{"context_source", "context_candidate", "context_read", "context_use", "turn_offset_ns", "本轮证据", "候选", "已读", "采用", "排除"} {
		if !strings.Contains(js, want) {
			t.Fatalf("room app missing context activation contract %q", want)
		}
	}
	for _, want := range []string{".event-dot.candidate", ".event-dot.read", ".event-dot.used", ".event-dot.dismissed", ".evidence-card"} {
		if !strings.Contains(css, want) {
			t.Fatalf("room styles missing lifecycle marker %q", want)
		}
	}
}
