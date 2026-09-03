package theater

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Store) SetPerformanceDirective(_ context.Context, sessionID string, expectedVersion int64, directive *PerformanceDirective) (*PerformanceDirective, RoleInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return nil, RoleInstance{}, err
	}
	if session.ID != sessionID {
		return nil, RoleInstance{}, fmt.Errorf("role session changed")
	}
	if expectedVersion > 0 && inst.Version != expectedVersion {
		return nil, RoleInstance{}, fmt.Errorf("role instance version conflict")
	}
	before := inst.Performance
	inst.Performance = directive
	inst.Version++
	inst.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return nil, RoleInstance{}, err
	}
	return before, inst, nil
}

func (s *Store) AdvanceStageTurn(_ context.Context, sessionID string) (RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, session, err := s.activeLocked()
	if err != nil {
		return RoleSession{}, err
	}
	if session.ID != sessionID || session.Status != SessionActive {
		return RoleSession{}, fmt.Errorf("role session changed or paused")
	}
	session.StageTurns++
	session.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleSession{}, err
	}
	return session, nil
}

func performanceDirectiveJSON(value *PerformanceDirective) string {
	if value == nil {
		return ""
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}

func parsePerformanceDirective(value string) *PerformanceDirective {
	if cleanText(value) == "" {
		return nil
	}
	var directive PerformanceDirective
	if json.Unmarshal([]byte(value), &directive) != nil {
		return nil
	}
	return &directive
}

func (s *Store) SetScene(_ context.Context, sessionID string, expectedVersion int64, scene string) (string, RoleInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return "", RoleInstance{}, err
	}
	if session.ID != sessionID {
		return "", RoleInstance{}, fmt.Errorf("role session changed")
	}
	if expectedVersion > 0 && inst.Version != expectedVersion {
		return "", RoleInstance{}, fmt.Errorf("role instance version conflict")
	}
	before := inst.Scene
	inst.Scene = cleanText(scene)
	inst.Version++
	inst.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return "", RoleInstance{}, err
	}
	return before, inst, nil
}

func (s *Store) SetInstanceState(_ context.Context, sessionID string, expectedVersion int64, key, value string) (string, RoleInstance, error) {
	key = cleanText(key)
	if key == "" {
		return "", RoleInstance{}, fmt.Errorf("state key required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return "", RoleInstance{}, err
	}
	if session.ID != sessionID {
		return "", RoleInstance{}, fmt.Errorf("role session changed")
	}
	if expectedVersion > 0 && inst.Version != expectedVersion {
		return "", RoleInstance{}, fmt.Errorf("role instance version conflict")
	}
	if inst.State == nil {
		inst.State = map[string]string{}
	}
	before := inst.State[key]
	if cleanText(value) == "" {
		delete(inst.State, key)
	} else {
		inst.State[key] = cleanText(value)
	}
	inst.Version++
	inst.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return "", RoleInstance{}, err
	}
	return before, inst, nil
}

func (s *Store) SetWorldlineState(_ context.Context, sessionID string, expectedVersion int64, key, value string) (string, RoleWorldline, error) {
	key = cleanText(key)
	if key == "" {
		return "", RoleWorldline{}, fmt.Errorf("world state key required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, session, err := s.activeLocked()
	if err != nil {
		return "", RoleWorldline{}, err
	}
	if session.ID != sessionID {
		return "", RoleWorldline{}, fmt.Errorf("role session changed")
	}
	var world RoleWorldline
	if err := readJSON(s.worldlinePath(session.WorldlineID), &world); err != nil {
		return "", RoleWorldline{}, err
	}
	if expectedVersion > 0 && world.Version != expectedVersion {
		return "", RoleWorldline{}, fmt.Errorf("worldline version conflict")
	}
	if world.State == nil {
		world.State = map[string]string{}
	}
	before := world.State[key]
	if cleanText(value) == "" {
		delete(world.State, key)
	} else {
		world.State[key] = cleanText(value)
	}
	world.Version++
	world.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.worldlinePath(world.ID), world); err != nil {
		return "", RoleWorldline{}, err
	}
	return before, world, nil
}

func (s *Store) SetMemoryMasked(_ context.Context, sessionID, episodeID string, masked bool) (bool, RoleWorldline, error) {
	episodeID = cleanText(episodeID)
	if episodeID == "" {
		return false, RoleWorldline{}, fmt.Errorf("episode id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, session, err := s.activeLocked()
	if err != nil {
		return false, RoleWorldline{}, err
	}
	if session.ID != sessionID {
		return false, RoleWorldline{}, fmt.Errorf("role session changed")
	}
	var world RoleWorldline
	if err := readJSON(s.worldlinePath(session.WorldlineID), &world); err != nil {
		return false, RoleWorldline{}, err
	}
	before := containsString(world.MaskedMemoryIDs, episodeID)
	if masked {
		world.MaskedMemoryIDs = appendUnique(world.MaskedMemoryIDs, episodeID)
	} else {
		world.MaskedMemoryIDs = removeString(world.MaskedMemoryIDs, episodeID)
	}
	world.Version++
	world.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.worldlinePath(world.ID), world); err != nil {
		return false, RoleWorldline{}, err
	}
	return before, world, nil
}

func (s *Store) SwitchWorldline(_ context.Context, sessionID, worldlineID string) (RoleInstance, RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return RoleInstance{}, RoleSession{}, err
	}
	if session.ID != sessionID {
		return RoleInstance{}, RoleSession{}, fmt.Errorf("role session changed")
	}
	var world RoleWorldline
	if err := readJSON(s.worldlinePath(worldlineID), &world); err != nil {
		return RoleInstance{}, RoleSession{}, err
	}
	if world.RoleInstanceID != inst.ID {
		return RoleInstance{}, RoleSession{}, fmt.Errorf("worldline belongs to another role instance")
	}
	now := time.Now().UTC()
	inst.CurrentWorldlineID, inst.Version, inst.UpdatedAt = world.ID, inst.Version+1, now
	session.WorldlineID, session.UpdatedAt = world.ID, now
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return RoleInstance{}, RoleSession{}, err
	}
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleInstance{}, RoleSession{}, err
	}
	return inst, session, nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func removeString(items []string, target string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != target {
			out = append(out, item)
		}
	}
	return out
}
