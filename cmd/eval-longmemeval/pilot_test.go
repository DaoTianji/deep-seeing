package main

import (
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"testing"
)

func TestPilotScopeIsolation(t *testing.T) {
	a := pilotScope(Input{Question: "a"})
	b := pilotScope(Input{Question: "b"})
	if a == b || a == (identity.TenantScope{UserID: "mudnet", AgentID: "deep-seeing"}) {
		t.Fatal("pilot scope collision")
	}
	if memory.HindsightBank(a) == memory.HindsightBank(b) {
		t.Fatal("bank collision")
	}
}
