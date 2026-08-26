package runtime

import "testing"

func TestParseRecallMode(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  RecallMode
		valid bool
	}{
		{name: "empty defaults legacy", want: RecallModeLegacy, valid: true},
		{name: "legacy", raw: " LEGACY ", want: RecallModeLegacy, valid: true},
		{name: "agent", raw: "Agent", want: RecallModeAgent, valid: true},
		{name: "invalid", raw: "hybrid", want: RecallModeLegacy, valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := ParseRecallMode(tt.raw)
			if got != tt.want || valid != tt.valid {
				t.Fatalf("ParseRecallMode(%q) = (%q, %v), want (%q, %v)", tt.raw, got, valid, tt.want, tt.valid)
			}
		})
	}
}
