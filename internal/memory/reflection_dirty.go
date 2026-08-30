package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReflectionDirtyState struct {
	Dirty     bool      `json:"dirty"`
	Reasons   []string  `json:"reasons,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *ReflectionStore) MarkDirty(personID, reason string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.markDirtyLocked(personID, reason)
}

func (s *ReflectionStore) IsDirty(personID string) (ReflectionDirtyState, error) {
	if s == nil {
		return ReflectionDirtyState{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readDirtyLocked()
	if err != nil {
		return ReflectionDirtyState{}, err
	}
	return all[strings.TrimSpace(personID)], nil
}

func (s *ReflectionStore) ClearDirty(personID string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readDirtyLocked()
	if err != nil {
		return err
	}
	delete(all, strings.TrimSpace(personID))
	return writeJSONAtomic(filepath.Join(s.dir, "dirty.json"), all)
}

func (s *ReflectionStore) markDirtyLocked(personID, reason string) error {
	personID = strings.TrimSpace(personID)
	if personID == "" {
		return nil
	}
	all, err := s.readDirtyLocked()
	if err != nil {
		return err
	}
	state := all[personID]
	state.Dirty, state.UpdatedAt = true, time.Now().UTC()
	reason = strings.TrimSpace(reason)
	if reason != "" {
		state.Reasons = uniqueStrings(append(state.Reasons, reason))
		if len(state.Reasons) > 8 {
			state.Reasons = state.Reasons[len(state.Reasons)-8:]
		}
	}
	all[personID] = state
	return writeJSONAtomic(filepath.Join(s.dir, "dirty.json"), all)
}

func (s *ReflectionStore) readDirtyLocked() (map[string]ReflectionDirtyState, error) {
	all := map[string]ReflectionDirtyState{}
	raw, err := os.ReadFile(filepath.Join(s.dir, "dirty.json"))
	if os.IsNotExist(err) {
		return all, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return all, nil
	}
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	return all, nil
}
