package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// ExecutionQueue serializes cognitive turns for one agent.
// Chat, Session Review, Dream, and future autonomous turns must enter via RunCognitive.
// Pure IO (e.g. future web fetch) stays outside this queue.
type ExecutionQueue struct {
	agentID string
	mu      sync.Mutex
}

// NewExecutionQueue binds a queue to an agent_id.
func NewExecutionQueue(agentID string) *ExecutionQueue {
	id := strings.TrimSpace(agentID)
	if id == "" {
		id = "local"
	}
	return &ExecutionQueue{agentID: id}
}

// AgentID returns the queue owner.
func (q *ExecutionQueue) AgentID() string {
	if q == nil {
		return ""
	}
	return q.agentID
}

// RunCognitive runs fn under the per-agent cognitive lock.
// name is a short label for debugging (chat|review|dream|auto|…).
func (q *ExecutionQueue) RunCognitive(ctx context.Context, name string, fn func(context.Context) error) error {
	if q == nil {
		return fmt.Errorf("execution queue required")
	}
	if fn == nil {
		return fmt.Errorf("cognitive fn required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = name // reserved for future tracing
	return fn(ctx)
}

// AutoSessionID builds the P7 autonomous session naming convention.
// Format: auto:<intent_id>:<attempt>
func AutoSessionID(intentID string, attempt int) string {
	id := strings.TrimSpace(intentID)
	if id == "" {
		id = "unknown"
	}
	id = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == ':':
			return r
		default:
			return '_'
		}
	}, id)
	if attempt < 1 {
		attempt = 1
	}
	return "auto:" + id + ":" + strconv.Itoa(attempt)
}

// IsAutoSession reports whether session_id follows the auto naming convention.
func IsAutoSession(sessionID string) bool {
	return strings.HasPrefix(strings.TrimSpace(sessionID), "auto:")
}
