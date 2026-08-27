package runtime

import (
	"strings"
	"testing"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
)

func TestFormatAttentionSnapshotContainsOnlyStructuredLeads(t *testing.T) {
	text := formatAttentionSnapshot(attention.Snapshot{
		Version: attention.Version, Capacity: attention.DefaultCapacity(), Items: []attention.Item{{
			Source: contextsource.Episode, ID: "ep_1", Role: contextsource.Evidence, Tier: attention.Center, IdleTurns: 2,
		}},
	})
	for _, want := range []string{"center=4", "episode:ep_1", "role=evidence", "idle_turns=2", "仍须调用对应 read"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "SECRET_BODY") {
		t.Fatal("attention snapshot leaked source body")
	}
}
