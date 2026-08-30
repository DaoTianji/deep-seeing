package memory

import "sync"

// ReflectionLiveState is a public progress snapshot. It mirrors the durable
// ReflectionRun shape and never contains episode bodies or hidden reasoning.
type ReflectionLiveState struct {
	Phase   string        `json:"phase"`
	Running bool          `json:"running"`
	Run     ReflectionRun `json:"run"`
}

type ReflectionLiveStore struct {
	mu    sync.RWMutex
	state ReflectionLiveState
}

func (s *ReflectionLiveStore) Update(run ReflectionRun, phase string, running bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.state = ReflectionLiveState{Phase: phase, Running: running, Run: run}
	s.mu.Unlock()
}

func (s *ReflectionLiveStore) Snapshot() ReflectionLiveState {
	if s == nil {
		return ReflectionLiveState{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}
