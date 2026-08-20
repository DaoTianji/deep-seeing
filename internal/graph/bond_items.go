package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/identity"
)

// Slot item caps (T1 frozen defaults).
const (
	MaxItemsBasics      = 20 // treated as soft; basics often field-like
	MaxItemsInteraction = 15
	MaxItemsBoundaries  = 20
	MaxItemsPriorities  = 15
	MaxItemsBaseline    = 10
)

// NormalizeSlot maps legacy field names to frozen slot IDs.
func NormalizeSlot(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "basics", "basics_fact":
		return SlotBasics, nil
	case "interaction", "style":
		return SlotInteraction, nil
	case "boundaries", "boundary", "bound":
		return SlotBoundaries, nil
	case "priorities", "concerns", "concern":
		return SlotPriorities, nil
	case "baseline":
		return SlotBaseline, nil
	case "strategy":
		return "", fmt.Errorf("strategy is derived-only and not a Bond SoT slot")
	case "":
		return "", fmt.Errorf("slot required")
	default:
		return "", fmt.Errorf("unknown slot %q (basics|interaction|boundaries|priorities|baseline)", raw)
	}
}

// SlotItemLimit returns max active items for a slot.
func SlotItemLimit(slot string) int {
	switch slot {
	case SlotBasics:
		return MaxItemsBasics
	case SlotInteraction:
		return MaxItemsInteraction
	case SlotBoundaries:
		return MaxItemsBoundaries
	case SlotPriorities:
		return MaxItemsPriorities
	case SlotBaseline:
		return MaxItemsBaseline
	default:
		return 10
	}
}

// EffectiveItems returns stored Items, or legacy prose projection when Items empty.
func EffectiveItems(b Bond) []BondItem {
	if len(b.Items) > 0 {
		return b.Items
	}
	return LegacyItemsFromBond(b)
}

// EncodeItemsJSON serializes items for Neo4j.
func EncodeItemsJSON(items []BondItem) (string, error) {
	if items == nil {
		items = []BondItem{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "[]", err
	}
	return string(raw), nil
}

// DecodeItemsJSON parses items_json; empty → nil slice.
func DecodeItemsJSON(raw string) ([]BondItem, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var items []BondItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	return items, nil
}

// MaterializeItems ensures Bond.Items is populated from legacy prose when needed.
func MaterializeItems(b Bond) Bond {
	if len(b.Items) > 0 {
		return b
	}
	b.Items = LegacyItemsFromBond(b)
	return b
}

// countActiveInSlot counts active items in a slot.
func countActiveInSlot(items []BondItem, slot string) int {
	n := 0
	for _, it := range items {
		if it.Slot != slot {
			continue
		}
		if it.Status == "" || it.Status == "active" {
			n++
		}
	}
	return n
}

// NewBondItemID allocates an item id.
func NewBondItemID(slot string) string {
	return fmt.Sprintf("%s:%s", slot, uuid.NewString()[:8])
}

// AppendItemSpec is the write payload for one claim.
type AppendItemSpec struct {
	Slot            string
	Claim           string
	Source          string // explicit|observed|legacy|fast_write
	SourceEpisodeID string
	FastWrite       bool // must be boundaries-only
}

// PrepareAppendItem validates and builds the new item list + mirrored prose.
func PrepareAppendItem(cur Bond, spec AppendItemSpec) (Bond, BondItem, error) {
	slot, err := NormalizeSlot(spec.Slot)
	if err != nil {
		return Bond{}, BondItem{}, err
	}
	claim := strings.TrimSpace(spec.Claim)
	if claim == "" {
		return Bond{}, BondItem{}, fmt.Errorf("claim required")
	}
	if utf8Len(claim) > ItemClaimMaxRunes*4 {
		// hard stop absurd payloads; inject still clips at ItemClaimMaxRunes
		claim = clipRunes(claim, ItemClaimMaxRunes*4)
	}
	if spec.FastWrite && slot != SlotBoundaries {
		return Bond{}, BondItem{}, fmt.Errorf("fast_write only allowed for boundaries")
	}
	cur = MaterializeItems(cur)
	if countActiveInSlot(cur.Items, slot) >= SlotItemLimit(slot) {
		return Bond{}, BondItem{}, fmt.Errorf("slot %s at item limit (%d); propose consolidation", slot, SlotItemLimit(slot))
	}
	source := strings.TrimSpace(spec.Source)
	if source == "" {
		if spec.FastWrite {
			source = "fast_write"
		} else {
			source = "explicit"
		}
	}
	item := BondItem{
		ID:     NewBondItemID(slot),
		Slot:   slot,
		Claim:  claim,
		Source: source,
		Status: "active",
	}
	// dedupe exact claim in same slot
	for _, existing := range cur.Items {
		if existing.Slot == slot && (existing.Status == "" || existing.Status == "active") &&
			strings.TrimSpace(existing.Claim) == claim {
			return cur, existing, nil
		}
	}
	cur.Items = append(cur.Items, item)
	cur.Version++
	if spec.SourceEpisodeID != "" {
		cur.SourceEpisodeIDs = MergeSourceIDs(cur.SourceEpisodeIDs, spec.SourceEpisodeID)
	}
	mirrorProse(&cur, slot, claim)
	return cur, item, nil
}

func mirrorProse(b *Bond, slot, claim string) {
	switch slot {
	case SlotBasics:
		b.Basics = MergeMedium(b.Basics, claim)
	case SlotInteraction:
		v, _ := ApplyHighField(b.Style, claim, "append")
		b.Style = v
	case SlotBoundaries:
		v, _ := ApplyHighField(b.Boundaries, claim, "append")
		b.Boundaries = v
	case SlotPriorities:
		b.Concerns = MergeMedium(b.Concerns, claim)
	case SlotBaseline:
		b.Baseline = MergeMedium(b.Baseline, claim)
	}
}

func utf8Len(s string) int {
	return len([]rune(s))
}

// PersistBondState writes items + mirrored prose + version.
func (s *Store) PersistBondState(ctx context.Context, scope identity.TenantScope, personID string, b Bond) (Bond, error) {
	if s == nil {
		return Bond{}, fmt.Errorf("neo4j: nil store")
	}
	if err := scope.Validate(); err != nil {
		return Bond{}, err
	}
	personID = normalizePersonID(scope, personID)
	if err := s.ensurePersonBond(ctx, scope.AgentID, personID); err != nil {
		return Bond{}, err
	}
	itemsJSON, err := EncodeItemsJSON(b.Items)
	if err != nil {
		return Bond{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	conf := b.Confidence
	if conf < 0.2 {
		conf = 0.2
	}
	// strategy prose is retained for migration visibility but not treated as SoT on read
	err = s.write(ctx, `
MATCH (self:Self {id: $self_id})-[b:BOND]->(p:Person {id: $person_id})
SET b.basics = $basics,
    b.concerns = $concerns,
    b.baseline = $baseline,
    b.strategy = $strategy,
    b.style = $style,
    b.boundaries = $boundaries,
    b.items_json = $items_json,
    b.bond_version = $bond_version,
    b.strategy_cache = $strategy_cache,
    b.strategy_cache_version = $strategy_cache_version,
    b.confidence = $confidence,
    b.last_confirmed_at = $now,
    b.source_episode_ids = $sources,
    b.updated_at = $now
`, map[string]any{
		"self_id":                 scope.AgentID,
		"person_id":               personID,
		"basics":                  b.Basics,
		"concerns":                b.Concerns,
		"baseline":                b.Baseline,
		"strategy":                b.Strategy,
		"style":                   b.Style,
		"boundaries":              b.Boundaries,
		"items_json":              itemsJSON,
		"bond_version":            b.Version,
		"strategy_cache":          b.StrategyCache,
		"strategy_cache_version":  b.StrategyCacheVer,
		"confidence":              conf,
		"now":                     now,
		"sources":                 b.SourceEpisodeIDs,
	})
	if err != nil {
		return Bond{}, err
	}
	return s.GetBond(ctx, scope, personID)
}

// AppendBondItem appends one claim into a slot (SoT path for T1-Write).
func (s *Store) AppendBondItem(ctx context.Context, scope identity.TenantScope, personID string, spec AppendItemSpec) (Bond, BondItem, error) {
	cur, err := s.GetBond(ctx, scope, personID)
	if err != nil {
		return Bond{}, BondItem{}, err
	}
	next, item, err := PrepareAppendItem(cur, spec)
	if err != nil {
		return Bond{}, BondItem{}, err
	}
	// exact duplicate short-circuit: no version bump rewrite needed if unchanged length? still ok to persist
	saved, err := s.PersistBondState(ctx, scope, personID, next)
	if err != nil {
		return Bond{}, BondItem{}, err
	}
	return saved, item, nil
}

// SetBondStrategyCache stores a derived strategy view bound to the current bond_version.
// Does not bump Version (cache refresh only).
func (s *Store) SetBondStrategyCache(ctx context.Context, scope identity.TenantScope, personID, text string) (Bond, error) {
	cur, err := s.GetBond(ctx, scope, personID)
	if err != nil {
		return Bond{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Bond{}, fmt.Errorf("strategy cache text required")
	}
	if utf8Len(text) > StrategyMaxRunes*2 {
		text = clipRunes(text, StrategyMaxRunes*2)
	}
	cur.StrategyCache = text
	cur.StrategyCacheVer = cur.Version
	return s.PersistBondState(ctx, scope, personID, cur)
}
