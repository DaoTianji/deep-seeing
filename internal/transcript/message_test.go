package transcript_test

import (
	"deep-seeing/internal/transcript"
	"testing"
)

func TestTurnMessagesSharePublicTurnID(t *testing.T) {
	user := transcript.UserTurn("hello", "turn-1")
	assistant := transcript.AssistantTurn("hi", "turn-1")
	if user.TurnID != "turn-1" || assistant.TurnID != user.TurnID {
		t.Fatalf("turn ids=%q/%q", user.TurnID, assistant.TurnID)
	}
	if transcript.User("legacy").TurnID != "" {
		t.Fatal("legacy constructor unexpectedly sets turn id")
	}
}
