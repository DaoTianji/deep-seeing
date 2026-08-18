package graph_test

import (
	"testing"

	"deep-seeing/internal/graph"
)

func TestApplyHighFieldAppendAndRejectReplace(t *testing.T) {
	got, err := graph.ApplyHighField("直接", "偶尔委婉", "append")
	if err != nil {
		t.Fatal(err)
	}
	if got != "直接\n偶尔委婉" {
		t.Fatalf("got %q", got)
	}

	_, err = graph.ApplyHighField("直接", "完全换掉", "replace")
	if err == nil {
		t.Fatal("expected replace rejected")
	}

	got, err = graph.ApplyHighField("", "新边界", "replace")
	if err != nil {
		t.Fatal(err)
	}
	if got != "新边界" {
		t.Fatalf("empty replace got %q", got)
	}
}

func TestSummaryFromContent(t *testing.T) {
	s := graph.SummaryFromContent("第一行\n第二行", 10)
	if s != "第一行" {
		t.Fatalf("got %q", s)
	}
}
