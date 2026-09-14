package story

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestEndingReviewRequiresAllPublicFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*endingReview)
	}{
		{"missing", func(r *endingReview) { r.Fields = r.Fields[:3] }},
		{"duplicate", func(r *endingReview) { r.Fields[3].Field = "story" }},
		{"unknown", func(r *endingReview) { r.Fields[3].Field = "style" }},
		{"missing_boolean", func(r *endingReview) { r.Fields[0].OK = nil }},
		{"missing_verdict", func(r *endingReview) { r.Fields[0].Reason = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r endingReview
			_ = json.Unmarshal([]byte(endingOK), &r)
			tc.mutate(&r)
			if ok, _, err := r.verdict(); ok || err == nil {
				t.Fatal("incomplete field audit passed")
			}
		})
	}
	var r endingReview
	_ = json.Unmarshal([]byte(endingOK), &r)
	if ok, _, err := r.verdict(); !ok || err != nil {
		t.Fatal("complete positive audit rejected")
	}
	no := false
	r.Fields[0].OK, r.Fields[0].Reason = &no, "告知价格没有免除责任"
	if ok, reason, err := r.verdict(); ok || err != nil || !strings.Contains(reason, "resolution") {
		t.Fatal("global approval overrode a failed resolution audit")
	}
}

func TestIncompleteEndingAuditDoesNotRewriteOrCommit(t *testing.T) {
	e, c, o, b := setup(t)
	b = seedVisitor(t, e, b)
	c.replies = []string{closingDecision(t, b.Turns[0].ID), judgeOK, `{"ok":true}`}
	_, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), b.Revision, false, true, 10)
	if err == nil || len(c.calls) != 3 {
		t.Fatal("missing audit caused publication or a pointless prose rewrite")
	}
	got, _ := e.Get(o, b.ID)
	if got.Revision != b.Revision || got.Ending != nil {
		t.Fatal("incomplete audit changed the story")
	}
}

func TestObservationAuditCannotHideBehindGlobalApproval(t *testing.T) {
	for _, verdict := range []string{
		`{"ok":true}`,
		`{"ok":true,"observation_check":{"field":"observation","ok":false,"reason":"观察把外形相同改成了赝品"}}`,
	} {
		e, c, owner, b := setup(t, actorOK, decisionOK, verdict)
		_, err := e.Turn(context.Background(), owner, b.ID, "mathilde", "请考虑坦白", uuid.NewString(), b.Revision, false)
		if err == nil || len(c.calls) != 3 {
			t.Fatal("incomplete/failed public observation approved")
		}
		got, _ := e.Get(owner, b.ID)
		if got.Revision != b.Revision || len(got.Turns) != 0 || len(got.Memories) != 0 {
			t.Fatal("bad observation partially saved")
		}
	}
}
