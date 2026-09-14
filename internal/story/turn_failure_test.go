package story

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRejectedTurnKeepsBoundedPrivateVerdictNotCandidateMemory(t *testing.T) {
	e, c, o, b := setup(t)
	b = seedVisitor(t, e, b)
	dir := filepath.Join(e.Root, o, "diagnostics")
	for i := 0; i < 2; i++ {
		var d Decision
		_ = json.Unmarshal([]byte(closingDecision(t, b.Turns[0].ID)), &d)
		d.Ending.Story += "REJECTED_CANDIDATE_BODY"
		raw, _ := json.Marshal(d)
		verdict, _ := json.Marshal(map[string]any{"ok": false, "reason": strings.Repeat("过去没有发生这件事。", 100)})
		c.replies = []string{string(raw), string(verdict)}
		req := uuid.NewString()
		_, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", req, b.Revision, false, true, 10)
		if err == nil {
			t.Fatal("expected rejection")
		}
		raw, err = os.ReadFile(filepath.Join(dir, b.ID+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var trace turnFailure
		if err := json.Unmarshal(raw, &trace); err != nil {
			t.Fatal(err)
		}
		if trace.Phase != "continuity" || trace.Revision != b.Revision || trace.RequestID != req || len([]rune(trace.Verdict)) != 400 {
			t.Fatal("incorrect verdict trace")
		}
		if strings.Contains(string(raw), "REJECTED_CANDIDATE_BODY") {
			t.Fatal("candidate prose persisted")
		}
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("unbounded diagnostic history")
	}
	info, _ := os.Stat(filepath.Join(dir, b.ID+".json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("diagnostic not private")
	}
	dirInfo, _ := os.Stat(dir)
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatal("directory not private")
	}
	got, _ := e.Get(o, b.ID)
	if got.Revision != b.Revision || got.Ending != nil || len(got.Turns) != len(b.Turns) {
		t.Fatal("failed candidate became story memory")
	}
	snap, _ := e.Snapshot(got, "mathilde")
	raw, _ := json.Marshal(snap)
	if strings.Contains(string(raw), "过去没有发生") {
		t.Fatal("verdict entered actor context")
	}
	if err := e.saveTurnFailure("../bad", b.ID, uuid.NewString(), 1, "validation", ""); err == nil {
		t.Fatal("unsafe diagnostic path accepted")
	}
}
