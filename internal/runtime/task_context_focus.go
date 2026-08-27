package runtime

import (
	"strings"
	"sync"

	"deep-seeing/internal/observe"
)

// TaskContextFocusReader exposes session-only task focus to snapshot builders and tools.
type TaskContextFocusReader interface {
	Current(sessionID string) (workspaceID, intentID string)
}

// TaskContextFocusController stores a selected focus for the lifetime of a process/session.
// It is intentionally not backed by Episode, Workspace, Intent, or any other LTM.
type TaskContextFocusController interface {
	TaskContextFocusReader
	Apply(sessionID string, event observe.TaskContextFocusTrace)
}

type taskContextFocusState struct {
	workspaceID string
	intentID    string
}

// SessionTaskContextFocusStore is an in-memory focus cache keyed by session.
type SessionTaskContextFocusStore struct {
	mu     sync.RWMutex
	states map[string]taskContextFocusState
}

func NewSessionTaskContextFocusStore() *SessionTaskContextFocusStore {
	return &SessionTaskContextFocusStore{states: map[string]taskContextFocusState{}}
}

func (s *SessionTaskContextFocusStore) Current(sessionID string) (string, string) {
	if s == nil {
		return "", ""
	}
	sessionID = strings.TrimSpace(sessionID)
	s.mu.RLock()
	state := s.states[sessionID]
	s.mu.RUnlock()
	return state.workspaceID, state.intentID
}

func (s *SessionTaskContextFocusStore) Apply(sessionID string, event observe.TaskContextFocusTrace) {
	if s == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch event.Action {
	case "continue", "switch":
		s.states[sessionID] = taskContextFocusState{
			workspaceID: strings.TrimSpace(event.WorkspaceID),
			intentID:    strings.TrimSpace(event.IntentID),
		}
	case "clear":
		delete(s.states, sessionID)
	}
}
