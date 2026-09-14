package story

import (
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestRecordsAndSigning(t *testing.T) {
	e, _, owner, b := setup(t)
	if _, err := e.Sign(owner, b.ID, "读者", b.Revision); err == nil {
		t.Fatal("signed unfinished story")
	}
	b.Ending = endingFixture("")
	b.Ending.Contributions = nil
	if err := e.save(b); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(b.Ending)
	for _, name := range []string{"", strings.Repeat("字", 41), "A\nB"} {
		if _, err := e.Sign(owner, b.ID, name, b.Revision); err == nil {
			t.Fatal("invalid name")
		}
	}
	if _, err := e.Sign(uuid.NewString(), b.ID, "别人", b.Revision); err == nil {
		t.Fatal("other visitor signed")
	}
	if _, err := e.Sign(owner, b.ID, "读者", b.Revision+1); err == nil {
		t.Fatal("stale sign")
	}
	signed, err := e.Sign(owner, b.ID, "  读者甲  ", b.Revision)
	if err != nil || signed.Signature != "读者甲" || signed.SignedAt.IsZero() {
		t.Fatal(err)
	}
	after, _ := json.Marshal(signed.Ending)
	if string(before) != string(after) || signed.Revision != b.Revision {
		t.Fatal("narrative changed")
	}
	if _, err := e.Sign(owner, b.ID, "读者甲", b.Revision); err != nil {
		t.Fatal("retry failed", err)
	}
	if _, err := e.Sign(owner, b.ID, "读者乙", b.Revision); err == nil {
		t.Fatal("overwrote signature")
	}
	fresh, _ := New(e.Root, nil, "test", "agent")
	records := fresh.Records(owner)
	if len(records) != 1 || records[0].Signature != "读者甲" || !records[0].Completed {
		t.Fatal("lost record")
	}
	if len(fresh.Records(uuid.NewString())) != 0 || len(fresh.Records("../")) != 0 {
		t.Fatal("record leak")
	}
	child, err := fresh.Fork(owner, b.ID, b.Revision)
	if err != nil || child.Signature != "" || child.Ending != nil {
		t.Fatal("fork inherited signature")
	}
}
