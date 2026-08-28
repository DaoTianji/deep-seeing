package room

import (
	"strings"
	"testing"
	"time"

	"deep-seeing/internal/observe"
)

func TestNormalizeTurnTraceKeepsNewIDAndBackfillsLegacy(t *testing.T) {
	at := time.Date(2026, 8, 28, 10, 0, 0, 12, time.UTC)
	current := normalizeTurnTrace(observe.TurnTrace{TurnID: "turn-1", Timestamp: at, StartedAt: at})
	if current.TurnID != "turn-1" || !current.StartedAt.Equal(at) {
		t.Fatalf("current trace changed: %+v", current)
	}

	legacy := normalizeTurnTrace(observe.TurnTrace{Timestamp: at, RecallReads: []observe.RecallReadTrace{{TurnOffset: 3 * time.Second, Duration: time.Second}}})
	if !strings.HasPrefix(legacy.TurnID, "legacy-") {
		t.Fatalf("legacy id=%q", legacy.TurnID)
	}
	if !legacy.StartedAt.Equal(at) || !legacy.CompletedAt.Equal(at.Add(4*time.Second)) || legacy.Duration != 4*time.Second {
		t.Fatalf("legacy times=%s/%s", legacy.StartedAt, legacy.CompletedAt)
	}
	if again := normalizeTurnTrace(legacy); again.TurnID != legacy.TurnID {
		t.Fatalf("legacy id is not stable: %q != %q", again.TurnID, legacy.TurnID)
	}
}
