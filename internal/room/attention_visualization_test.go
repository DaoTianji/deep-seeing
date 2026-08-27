package room

import (
	"strings"
	"testing"
)

func TestRoomWebHandlesAttentionWorkspaceLiveAndReplay(t *testing.T) {
	data, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		`eventData.type === "attention_snapshot"`,
		`eventData.type === "attention_decision"`,
		"attentionByID",
		"trace.attention?.items",
		"trace.attention_decisions",
		"attention-ring",
		"中心 ${center} / 支撑 ${support} / 外围 ${periphery}",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("room app missing attention contract %q", want)
		}
	}
	styles, err := webFS.ReadFile("web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(styles)
	for _, want := range []string{".attention-ring.center", ".attention-ring.support", ".attention-ring.periphery"} {
		if !strings.Contains(css, want) {
			t.Fatalf("room styles missing attention tier %q", want)
		}
	}
	if strings.Contains(css, ".attention-ring .node-core") {
		t.Fatal("attention ring must not override source or lifecycle colors")
	}
}
