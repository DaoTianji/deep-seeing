package room

import (
	"strings"
	"testing"
)

func TestRoomWebHandlesUnifiedContextActivationAndLegacyReplay(t *testing.T) {
	data, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		`eventData.type === "context_source"`,
		`eventData.type === "context_candidate"`,
		`eventData.type === "context_read"`,
		`eventData.type === "context_use"`,
		"trace.context_candidates",
		"trace.context_reads",
		"trace.context_uses",
		"trace.recall_searches",
		"contextGraphView",
		"contextSourceGlyph",
		"context-focus",
		"回放上下文激活",
		"duration_ns",
		"turn_offset_ns",
		"召回耗时",
		"证据就绪",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("room app missing multi-source activation contract %q", want)
		}
	}
	if !strings.Contains(source, `status += " context-focus"`) {
		t.Fatal("context focus must be additive to the candidate/read/use state")
	}
	lane := strings.Index(source, "const contextNodes =")
	pinned := strings.Index(source, "for (const [id, position] of state.graphPinned)")
	if lane < 0 || pinned < 0 || lane > pinned {
		t.Fatal("context nodes are not laid out before optional pinned-node overrides")
	}
	timingSummary := strings.Index(source, "if (searchDuration || readDuration || firstRecall || evidenceReady)")
	attentionSummary := strings.Index(source, "if (attentionItems.length || attentionFinalItems?.length || attentionDecisions.length)")
	if timingSummary < 0 || attentionSummary < 0 || timingSummary > attentionSummary {
		t.Fatal("recall timing summary must not depend on attention state")
	}
	if durationFormatter, timeFormatter := strings.Index(source, "function formatDurationNS"), strings.Index(source, "function formatTime"); durationFormatter < 0 || timeFormatter < 0 || durationFormatter > timeFormatter {
		t.Fatal("duration formatter must be available at module scope before formatTime")
	}
	server, err := webFS.ReadFile("web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	styles := string(server)
	for _, want := range []string{".node-core.SceneNorm", ".node-core.Workspace", ".node-core.Intent", ".node-core.Proposal", ".node-group.context-focus"} {
		if !strings.Contains(styles, want) {
			t.Fatalf("room styles missing source marker %q", want)
		}
	}
	if strings.Contains(styles, ".node-group.context-focus .node-core") {
		t.Fatal("context focus must not override the node state color")
	}
}
