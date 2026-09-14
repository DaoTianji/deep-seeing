package story

import "testing"

func TestExactSourceExcerpts(t *testing.T) {
	b := Necklace()
	if len(b.Excerpts) != 7 {
		t.Fatal("missing source cards")
	}
	for _, e := range b.Evidence {
		x := b.Excerpts[e.ID]
		if x.Text == "" || SourceText()[x.StartByte:x.EndByte] != x.Text {
			t.Fatal("invalid source location", e.ID)
		}
	}
}
