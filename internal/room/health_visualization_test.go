package room

import (
	"strings"
	"testing"
)

func TestRoomWebShowsStructuredTurnHealth(t *testing.T) {
	js, _ := builtFrontendAssets(t)
	for _, want := range []string{"health", "运行状态", "部分离线"} {
		if !strings.Contains(js, want) {
			t.Fatalf("room app missing health contract %q", want)
		}
	}
}
