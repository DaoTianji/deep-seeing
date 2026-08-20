package graph_test

import (
	"strings"
	"testing"

	"deep-seeing/internal/graph"
)

func TestPrepareAppendItemFastWriteBoundariesOnly(t *testing.T) {
	_, _, err := graph.PrepareAppendItem(graph.Bond{}, graph.AppendItemSpec{
		Slot: graph.SlotInteraction, Claim: "x", FastWrite: true,
	})
	if err == nil || !strings.Contains(err.Error(), "fast_write") {
		t.Fatalf("err=%v", err)
	}
}

func TestPrepareAppendItemRejectsStrategy(t *testing.T) {
	_, err := graph.NormalizeSlot("strategy")
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestPrepareAppendItemLimit(t *testing.T) {
	var items []graph.BondItem
	for i := 0; i < graph.MaxItemsBoundaries; i++ {
		items = append(items, graph.BondItem{
			ID: graph.NewBondItemID(graph.SlotBoundaries), Slot: graph.SlotBoundaries,
			Claim: "b", Status: "active",
		})
	}
	_, _, err := graph.PrepareAppendItem(graph.Bond{Items: items}, graph.AppendItemSpec{
		Slot: graph.SlotBoundaries, Claim: "overflow", FastWrite: true,
	})
	if err == nil || !strings.Contains(err.Error(), "item limit") {
		t.Fatalf("err=%v", err)
	}
}

func TestEffectiveItemsPrefersStored(t *testing.T) {
	b := graph.Bond{
		Style: "legacy style should ignore",
		Items: []graph.BondItem{{ID: "interaction:1", Slot: graph.SlotInteraction, Claim: "stored", Status: "active"}},
	}
	items := graph.EffectiveItems(b)
	if len(items) != 1 || items[0].Claim != "stored" {
		t.Fatalf("%+v", items)
	}
	text, _ := graph.FormatCompactRecall(b, "")
	if strings.Contains(text, "legacy style") || !strings.Contains(text, "stored") {
		t.Fatalf("%s", text)
	}
}
