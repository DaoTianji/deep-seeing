package memory

import (
	"sync"

	"deep-seeing/internal/transcript"
)

// SessionStore is short-term session history (in-process or Redis).
type SessionStore interface {
	Get(sessionID string) ([]transcript.Message, error)
	Append(sessionID string, msgs ...transcript.Message) error
	Replace(sessionID string, msgs []transcript.Message) error
}

// STM is in-process short-term session history with a message bound.
type STM struct {
	mu          sync.Mutex
	sessions    map[string][]transcript.Message
	MaxMessages int
}

// NewSTM returns an in-memory SessionStore. Default MaxMessages is 100 when <= 0.
func NewSTM(maxMessages int) *STM {
	if maxMessages <= 0 {
		maxMessages = 100
	}
	return &STM{
		sessions:    make(map[string][]transcript.Message),
		MaxMessages: maxMessages,
	}
}

func (s *STM) Get(sessionID string) ([]transcript.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.sessions[sessionID]
	out := make([]transcript.Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (s *STM) Append(sessionID string, msgs ...transcript.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := append(append([]transcript.Message(nil), s.sessions[sessionID]...), msgs...)
	s.sessions[sessionID] = trimToMax(cur, s.MaxMessages)
	return nil
}

func (s *STM) Replace(sessionID string, msgs []transcript.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := append([]transcript.Message(nil), msgs...)
	s.sessions[sessionID] = trimToMax(cur, s.MaxMessages)
	return nil
}

func trimToMax(msgs []transcript.Message, max int) []transcript.Message {
	if max <= 0 || len(msgs) <= max {
		return msgs
	}
	return msgs[len(msgs)-max:]
}
