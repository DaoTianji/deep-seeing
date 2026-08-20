package graph

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Slot IDs for Bond norm governance (T1 frozen names).
const (
	SlotBasics      = "basics"
	SlotInteraction = "interaction"
	SlotBoundaries  = "boundaries"
	SlotPriorities  = "priorities"
	SlotBaseline    = "baseline"
	SlotStrategy    = "strategy" // derived only — not Bond SoT
)

// T1-Read injection budgets (runes / counts).
const (
	BondPlaceholder     = "对该人常模尚薄，避免臆测人格；优先询问与观察。"
	MustInjectMaxRunes  = 800
	InteractionTopN     = 5
	ItemClaimMaxRunes   = 80
	ConditionalMaxRunes = 400
	StrategyMaxRunes    = 120 // reserved; T1-Read omits Strategy SoT
)

// BondItem is one stable claim inside a slot (read-path atom for T1).
type BondItem struct {
	ID     string
	Slot   string
	Claim  string
	Source string // explicit | observed | legacy
	Status string // active | retired
}

// BondInjectTrace records what compact injection included this turn.
type BondInjectTrace struct {
	Slots        []string
	ItemIDs      []string
	Placeholder  bool
	StrategyOmit bool
}

// LegacyItemsFromBond maps current prose Bond fields into slot items.
// Strategy is intentionally omitted from SoT items (no longer a truth source).
func LegacyItemsFromBond(b Bond) []BondItem {
	var out []BondItem
	addLines := func(slot, prose, source string) {
		prose = strings.TrimSpace(prose)
		if prose == "" {
			return
		}
		lines := strings.Split(prose, "\n")
		idx := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			out = append(out, BondItem{
				ID:     fmt.Sprintf("%s:legacy:%d", slot, idx),
				Slot:   slot,
				Claim:  line,
				Source: source,
				Status: "active",
			})
			idx++
		}
	}
	if cn := strings.TrimSpace(b.CallName); cn != "" {
		out = append(out, BondItem{
			ID: SlotBasics + ":call_name", Slot: SlotBasics,
			Claim: "称呼偏好: " + cn, Source: "explicit", Status: "active",
		})
	}
	addLines(SlotBasics, b.Basics, "legacy")
	if role := strings.TrimSpace(b.RoleAtOrigin); role != "" {
		out = append(out, BondItem{
			ID: SlotBasics + ":role_at_origin", Slot: SlotBasics,
			Claim: "origin_role: " + role, Source: "legacy", Status: "active",
		})
	}
	addLines(SlotInteraction, b.Style, "legacy")
	addLines(SlotBoundaries, b.Boundaries, "legacy")
	addLines(SlotPriorities, b.Concerns, "legacy")
	addLines(SlotBaseline, b.Baseline, "legacy")
	// b.Strategy intentionally dropped as SoT
	return out
}

// FormatBondNormText renders the full norm for display (no injection budgets).
// Items are grouped by slot in frozen order; legacy prose is projected when Items is empty.
func FormatBondNormText(b Bond) string {
	active := filterActive(EffectiveItems(b))
	var sections []string
	for _, slot := range []string{SlotBasics, SlotInteraction, SlotBoundaries, SlotPriorities, SlotBaseline} {
		var lines []string
		for _, it := range active {
			if it.Slot != slot {
				continue
			}
			line := "- " + strings.TrimSpace(it.Claim)
			if src := strings.TrimSpace(it.Source); src != "" && src != "explicit" {
				line += "  （" + src + "）"
			}
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			sections = append(sections, "["+slot+"]\n"+strings.Join(lines, "\n"))
		}
	}
	if len(sections) == 0 {
		return BondPlaceholder
	}
	header := fmt.Sprintf("bond_version=%d · 共 %d 条结论", b.Version, len(active))
	body := header + "\n\n" + strings.Join(sections, "\n\n")
	if strat := strings.TrimSpace(b.StrategyCache); strat != "" {
		status := "stale，需重刷后才注入"
		if b.StrategyCacheVer == b.Version {
			status = "active，本轮会注入"
		}
		body += "\n\n[strategy · 派生缓存，非 SoT]\n- " + strat + "\n  （" + status + "）"
	}
	return body
}

// FormatCompactRecall builds T1 priority-truncated bond context for injection.
// query is used for conditional Priorities/Baseline keyword gating.
func FormatCompactRecall(b Bond, query string) (string, BondInjectTrace) {
	items := EffectiveItems(b)
	trace := BondInjectTrace{StrategyOmit: true}
	if len(items) == 0 && strings.TrimSpace(b.PersonID) == "" && strings.TrimSpace(b.RoleAtOrigin) == "" {
		// truly empty relational norm
	}
	active := filterActive(items)
	if len(active) == 0 {
		trace.Placeholder = true
		trace.Slots = []string{"placeholder"}
		return BondPlaceholder, trace
	}

	var sections []string
	must := collectSlots(active, SlotBasics, SlotBoundaries)
	mustText, mustIDs := renderMust(must, MustInjectMaxRunes)
	if mustText != "" {
		sections = append(sections, mustText)
		trace.Slots = appendUnique(trace.Slots, slotNames(must)...)
		trace.ItemIDs = append(trace.ItemIDs, mustIDs...)
	}

	inter := collectSlots(active, SlotInteraction)
	interText, interIDs := renderTopN(inter, InteractionTopN, ItemClaimMaxRunes, "Interaction")
	if interText != "" {
		sections = append(sections, interText)
		trace.Slots = appendUnique(trace.Slots, SlotInteraction)
		trace.ItemIDs = append(trace.ItemIDs, interIDs...)
	}

	cond := collectSlots(active, SlotPriorities, SlotBaseline)
	cond = filterKeywordHit(cond, query)
	condText, condIDs := renderBudget(cond, ConditionalMaxRunes, ItemClaimMaxRunes)
	if condText != "" {
		sections = append(sections, "## Conditional\n"+condText)
		trace.Slots = appendUnique(trace.Slots, slotNames(cond)...)
		trace.ItemIDs = append(trace.ItemIDs, condIDs...)
	}

	strat := strings.TrimSpace(b.StrategyCache)
	if strat != "" && b.StrategyCacheVer == b.Version {
		clipped := clipRunes(strat, StrategyMaxRunes)
		sections = append(sections, "## Strategy (derived cache)\n- "+clipped)
		trace.Slots = appendUnique(trace.Slots, SlotStrategy)
		trace.StrategyOmit = false
	}

	if len(sections) == 0 {
		trace.Placeholder = true
		trace.Slots = []string{"placeholder"}
		return BondPlaceholder, trace
	}

	header := "Bond compact（常模结论；细节以 Episode 为准）"
	if trace.StrategyOmit {
		header += "；Strategy 派生缓存未命中或未刷新"
	}
	if pid := strings.TrimSpace(b.PersonID); pid != "" {
		header += "\nperson: " + pid
	}
	return header + "\n\n" + strings.Join(sections, "\n\n"), trace
}

func filterActive(items []BondItem) []BondItem {
	var out []BondItem
	for _, it := range items {
		if it.Status == "" || it.Status == "active" {
			out = append(out, it)
		}
	}
	return out
}

func collectSlots(items []BondItem, slots ...string) []BondItem {
	want := map[string]bool{}
	for _, s := range slots {
		want[s] = true
	}
	var out []BondItem
	for _, it := range items {
		if want[it.Slot] {
			out = append(out, it)
		}
	}
	return out
}

func slotNames(items []BondItem) []string {
	var out []string
	seen := map[string]bool{}
	for _, it := range items {
		if !seen[it.Slot] {
			seen[it.Slot] = true
			out = append(out, it.Slot)
		}
	}
	return out
}

func renderMust(items []BondItem, maxRunes int) (string, []string) {
	if len(items) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("## Must (Basics / Boundaries)\n")
	var ids []string
	used := 0
	for _, it := range items {
		claim := clipRunes(it.Claim, ItemClaimMaxRunes)
		line := "- [" + it.Slot + "] " + claim
		n := utf8.RuneCountInString(line) + 1
		if used+n > maxRunes {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
		used += n
		ids = append(ids, it.ID)
	}
	return strings.TrimSpace(b.String()), ids
}

func renderTopN(items []BondItem, n, claimMax int, title string) (string, []string) {
	if len(items) == 0 || n <= 0 {
		return "", nil
	}
	if len(items) > n {
		items = items[:n]
	}
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteByte('\n')
	var ids []string
	for _, it := range items {
		b.WriteString("- ")
		b.WriteString(clipRunes(it.Claim, claimMax))
		b.WriteByte('\n')
		ids = append(ids, it.ID)
	}
	return strings.TrimSpace(b.String()), ids
}

func renderBudget(items []BondItem, maxTotal, claimMax int) (string, []string) {
	if len(items) == 0 {
		return "", nil
	}
	var b strings.Builder
	var ids []string
	used := 0
	for _, it := range items {
		claim := clipRunes(it.Claim, claimMax)
		line := "- [" + it.Slot + "] " + claim
		n := utf8.RuneCountInString(line) + 1
		if used+n > maxTotal {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
		used += n
		ids = append(ids, it.ID)
	}
	return strings.TrimSpace(b.String()), ids
}

func filterKeywordHit(items []BondItem, query string) []BondItem {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	tokens := keywordTokens(query)
	if len(tokens) == 0 {
		return nil
	}
	var out []BondItem
	for _, it := range items {
		claim := strings.ToLower(it.Claim)
		for _, tok := range tokens {
			if strings.Contains(claim, tok) {
				out = append(out, it)
				break
			}
		}
	}
	return out
}

func keywordTokens(query string) []string {
	// Split on whitespace and common punctuation; keep tokens length >= 2.
	fields := strings.FieldsFunc(query, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', ',', '，', '。', '.', '!', '？', '?', ';', '；', ':', '：', '/', '\\', '|':
			return true
		default:
			return false
		}
	})
	var out []string
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSpace(f))
		if utf8.RuneCountInString(f) < 2 {
			continue
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

func clipRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func appendUnique(dst []string, add ...string) []string {
	seen := map[string]bool{}
	for _, x := range dst {
		seen[x] = true
	}
	for _, x := range add {
		if x == "" || seen[x] {
			continue
		}
		seen[x] = true
		dst = append(dst, x)
	}
	return dst
}
