package attention

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"deep-seeing/internal/contextsource"
)

const Version = "1"

type Tier string

const (
	Center    Tier = "center"
	Support   Tier = "support"
	Periphery Tier = "periphery"
	Drop      Tier = "drop"
)

type Capacity struct {
	Center    int `json:"center"`
	Support   int `json:"support"`
	Periphery int `json:"periphery"`
}

func DefaultCapacity() Capacity {
	return Capacity{Center: 4, Support: 8, Periphery: 16}
}

type Item struct {
	Source    contextsource.Source `json:"source"`
	ID        string               `json:"id"`
	Role      contextsource.Role   `json:"role"`
	Tier      Tier                 `json:"tier"`
	IdleTurns int                  `json:"idle_turns,omitempty"`
}

type Snapshot struct {
	Version    string        `json:"version"`
	Revision   int64         `json:"revision"`
	Capacity   Capacity      `json:"capacity"`
	Items      []Item        `json:"items,omitempty"`
	TurnOffset time.Duration `json:"turn_offset_ns,omitempty"`
}

type Decision struct {
	Source        contextsource.Source `json:"source"`
	ID            string               `json:"id"`
	Target        Tier                 `json:"target"`
	ReplaceSource contextsource.Source `json:"replace_source,omitempty"`
	ReplaceID     string               `json:"replace_id,omitempty"`
}

type state struct {
	revision int64
	items    map[string]Item
}

// SessionStore is an in-memory attention workspace. It is intentionally not LTM.
// Capacity conflicts are rejected unless the Agent explicitly names a replacement.
type SessionStore struct {
	mu       sync.RWMutex
	capacity Capacity
	states   map[string]state
}

func NewSessionStore(capacity Capacity) *SessionStore {
	if capacity.Center <= 0 || capacity.Support <= 0 || capacity.Periphery <= 0 {
		capacity = DefaultCapacity()
	}
	return &SessionStore{capacity: capacity, states: map[string]state{}}
}

func (s *SessionStore) BeginTurn(sessionID string) Snapshot {
	if s == nil {
		return Snapshot{Version: Version, Capacity: DefaultCapacity()}
	}
	sessionID = strings.TrimSpace(sessionID)
	s.mu.Lock()
	st := cloneState(s.states[sessionID])
	if len(st.items) > 0 {
		st.revision++
		for key, item := range st.items {
			item.IdleTurns++
			st.items[key] = item
		}
		s.states[sessionID] = st
	}
	out := snapshotOf(st, s.capacity)
	s.mu.Unlock()
	return out
}

func (s *SessionStore) Snapshot(sessionID string) Snapshot {
	if s == nil {
		return Snapshot{Version: Version, Capacity: DefaultCapacity()}
	}
	s.mu.RLock()
	out := snapshotOf(s.states[strings.TrimSpace(sessionID)], s.capacity)
	s.mu.RUnlock()
	return out
}

// Touch marks an already retained item as active in the current turn. Reads
// and public use declarations call this automatically; it changes recency only,
// never tier, evidence role, or persistence.
func (s *SessionStore) Touch(sessionID string, source contextsource.Source, id string) bool {
	if s == nil || source == contextsource.Bond || !contextsource.ValidSource(source) {
		return false
	}
	sessionID = strings.TrimSpace(sessionID)
	id = strings.TrimSpace(id)
	if sessionID == "" || id == "" {
		return false
	}
	key := itemKey(source, id)
	s.mu.Lock()
	defer s.mu.Unlock()
	st := cloneState(s.states[sessionID])
	item, ok := st.items[key]
	if !ok || item.IdleTurns == 0 {
		return false
	}
	item.IdleTurns = 0
	st.items[key] = item
	st.revision++
	s.states[sessionID] = st
	return true
}

func (s *SessionStore) Apply(sessionID string, decisions []Decision) (Snapshot, error) {
	if s == nil {
		return Snapshot{}, fmt.Errorf("attention workspace unavailable")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || len(decisions) == 0 {
		return Snapshot{}, fmt.Errorf("session and decisions required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := cloneState(s.states[sessionID])
	seen := map[string]bool{}
	for _, raw := range decisions {
		decision, err := normalizeDecision(raw)
		if err != nil {
			return Snapshot{}, err
		}
		key := itemKey(decision.Source, decision.ID)
		if seen[key] {
			return Snapshot{}, fmt.Errorf("attention item %s appears more than once", key)
		}
		seen[key] = true
		if err := applyDecision(&st, s.capacity, decision); err != nil {
			return Snapshot{}, err
		}
	}
	st.revision++
	s.states[sessionID] = st
	return snapshotOf(st, s.capacity), nil
}

func normalizeDecision(in Decision) (Decision, error) {
	in.Source = contextsource.Source(strings.ToLower(strings.TrimSpace(string(in.Source))))
	in.ID = strings.TrimSpace(in.ID)
	in.Target = Tier(strings.ToLower(strings.TrimSpace(string(in.Target))))
	in.ReplaceSource = contextsource.Source(strings.ToLower(strings.TrimSpace(string(in.ReplaceSource))))
	in.ReplaceID = strings.TrimSpace(in.ReplaceID)
	if !contextsource.ValidSource(in.Source) || in.Source == contextsource.Bond || in.ID == "" {
		return Decision{}, fmt.Errorf("attention requires a non-Bond source and id")
	}
	if !validTarget(in.Target) {
		return Decision{}, fmt.Errorf("invalid attention target %q", in.Target)
	}
	if (in.ReplaceSource == "") != (in.ReplaceID == "") {
		return Decision{}, fmt.Errorf("replacement requires both source and id")
	}
	if in.ReplaceSource != "" && (!contextsource.ValidSource(in.ReplaceSource) || in.ReplaceSource == contextsource.Bond) {
		return Decision{}, fmt.Errorf("invalid replacement source %q", in.ReplaceSource)
	}
	return in, nil
}

func applyDecision(st *state, capacity Capacity, decision Decision) error {
	if st.items == nil {
		st.items = map[string]Item{}
	}
	key := itemKey(decision.Source, decision.ID)
	existing, exists := st.items[key]
	if decision.Target == Drop {
		if !exists {
			return fmt.Errorf("attention item %s is not in the workspace", key)
		}
		delete(st.items, key)
		return nil
	}

	if exists && existing.Tier == decision.Target {
		existing.IdleTurns = 0
		st.items[key] = existing
		return nil
	}
	if exists {
		delete(st.items, key)
	}
	if tierCount(st.items, decision.Target) >= tierCapacity(capacity, decision.Target) {
		replaceKey := itemKey(decision.ReplaceSource, decision.ReplaceID)
		replacement, ok := st.items[replaceKey]
		if decision.ReplaceSource == "" || !ok || replacement.Tier != decision.Target || replaceKey == key {
			if exists {
				st.items[key] = existing
			}
			return fmt.Errorf("attention %s is full; name one existing %s item to replace", decision.Target, decision.Target)
		}
		delete(st.items, replaceKey)
	} else if decision.ReplaceSource != "" {
		if exists {
			st.items[key] = existing
		}
		return fmt.Errorf("replacement is only valid when target tier is full")
	}
	st.items[key] = Item{
		Source: decision.Source, ID: decision.ID, Role: contextsource.RoleFor(decision.Source), Tier: decision.Target,
	}
	return nil
}

func snapshotOf(st state, capacity Capacity) Snapshot {
	out := Snapshot{Version: Version, Revision: st.revision, Capacity: capacity}
	for _, item := range st.items {
		out.Items = append(out.Items, item)
	}
	sort.Slice(out.Items, func(i, j int) bool {
		if tierOrder(out.Items[i].Tier) != tierOrder(out.Items[j].Tier) {
			return tierOrder(out.Items[i].Tier) < tierOrder(out.Items[j].Tier)
		}
		if out.Items[i].Source != out.Items[j].Source {
			return out.Items[i].Source < out.Items[j].Source
		}
		return out.Items[i].ID < out.Items[j].ID
	})
	return out
}

func cloneState(in state) state {
	out := state{revision: in.revision, items: map[string]Item{}}
	for key, item := range in.items {
		out.items[key] = item
	}
	return out
}

func itemKey(source contextsource.Source, id string) string { return string(source) + ":" + id }

func tierCount(items map[string]Item, tier Tier) int {
	count := 0
	for _, item := range items {
		if item.Tier == tier {
			count++
		}
	}
	return count
}

func tierCapacity(capacity Capacity, tier Tier) int {
	switch tier {
	case Center:
		return capacity.Center
	case Support:
		return capacity.Support
	case Periphery:
		return capacity.Periphery
	default:
		return 0
	}
}

func validTarget(tier Tier) bool {
	return tier == Center || tier == Support || tier == Periphery || tier == Drop
}

func tierOrder(tier Tier) int {
	switch tier {
	case Center:
		return 0
	case Support:
		return 1
	case Periphery:
		return 2
	default:
		return 3
	}
}
