package tools

import (
	"context"
	"path/filepath"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/runtime"
)

func TestContextFocusToolIsAgentModeOnly(t *testing.T) {
	episodes, err := memory.NewEpisodeStore(filepath.Join(t.TempDir(), "episodes"))
	if err != nil {
		t.Fatal(err)
	}
	focus := runtime.NewSessionTaskContextFocusStore()
	legacy, err := All(Deps{
		Scope: identity.LocalCLI(), Episodes: episodes, SessionID: "legacy",
		RecallMode: "legacy", TaskContextFocus: focus,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasNamedTool(t, legacy, "report_context_focus") {
		t.Fatal("legacy mode exposed report_context_focus")
	}
	agent, err := All(Deps{
		Scope: identity.LocalCLI(), Episodes: episodes, SessionID: "agent",
		RecallMode: "agent", TaskContextFocus: focus,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasNamedTool(t, agent, "report_context_focus") {
		t.Fatal("agent mode did not expose report_context_focus")
	}
}

func hasNamedTool(t *testing.T, all []einotool.BaseTool, name string) bool {
	t.Helper()
	for _, candidate := range all {
		info, err := candidate.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info != nil && info.Name == name {
			return true
		}
	}
	return false
}
