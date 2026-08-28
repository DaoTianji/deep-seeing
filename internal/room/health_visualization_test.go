package room

import (
	"strings"
	"testing"
)

func TestRoomWebShowsStructuredTurnHealth(t *testing.T) {
	app, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`eventData.type === "health"`, "trace.health?.status", "issue.code", "本轮已局部降级"} {
		if !strings.Contains(string(app), want) {
			t.Fatalf("room app missing health contract %q", want)
		}
	}
	mind, err := webFS.ReadFile("web/mind.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"trace.health?.status", "运行健康"} {
		if !strings.Contains(string(mind), want) {
			t.Fatalf("mind view missing health contract %q", want)
		}
	}
}
