package intent

import (
	"fmt"
	"strings"
	"time"
)

// Status is Intent lifecycle.
type Status string

const (
	StatusActive    Status = "active"
	StatusPaused    Status = "paused"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
)

// Trigger classifies why a wake happens.
type Trigger string

const (
	TriggerSelfScheduled Trigger = "SELF_SCHEDULED"
	TriggerTaskCompleted Trigger = "TASK_COMPLETED"
	TriggerMaintenance   Trigger = "MAINTENANCE"
	TriggerExternalEvent Trigger = "EXTERNAL_EVENT"
)

// Intent is a future commitment left by past Self.
type Intent struct {
	ID             string        `json:"id"`
	AgentID        string        `json:"agent_id"`
	Kind           IntentKind    `json:"kind"`
	Title          string        `json:"title"`
	Body           string        `json:"body,omitempty"`
	Status         Status        `json:"status"`
	Trigger        Trigger       `json:"trigger"`
	DueAt          time.Time     `json:"due_at"`
	Interval       time.Duration `json:"interval_seconds,omitempty"` // recurring period
	StaleAfterDays int           `json:"stale_after_days,omitempty"`
	AllowOutbound  bool          `json:"allow_outbound"` // default false — no proactive contact
	Attempt        int           `json:"attempt"`
	LastWakeAt     time.Time     `json:"last_wake_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// CreateInput creates a new Intent.
type CreateInput struct {
	AgentID        string
	Kind           IntentKind
	Title          string
	Body           string
	Trigger        Trigger
	DueAt          time.Time
	Interval       time.Duration
	StaleAfterDays int
	AllowOutbound  bool
}

// WakeJob is one scheduled/executed autonomous wake.
type WakeJob struct {
	ID          string    `json:"id"`
	IntentID    string    `json:"intent_id"`
	AgentID     string    `json:"agent_id"`
	Trigger     Trigger   `json:"trigger"`
	ScheduledAt time.Time `json:"scheduled_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	FinishedAt  time.Time `json:"finished_at,omitempty"`
	Status      string    `json:"status"` // pending|running|done|skipped|failed
	Decision    string    `json:"decision,omitempty"`
	Result      string    `json:"result,omitempty"` // sleep|noop|acted|cancelled
	SessionID   string    `json:"session_id,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// NormalizeKind maps raw kind.
func NormalizeKind(raw string) IntentKind {
	switch IntentKind(strings.ToLower(strings.TrimSpace(raw))) {
	case IntentOneShot:
		return IntentOneShot
	default:
		return IntentRecurring
	}
}

// NormalizeStatus maps raw status.
func NormalizeStatus(raw string) Status {
	switch Status(strings.ToLower(strings.TrimSpace(raw))) {
	case StatusPaused:
		return StatusPaused
	case StatusDone:
		return StatusDone
	case StatusCancelled:
		return StatusCancelled
	default:
		return StatusActive
	}
}

// NormalizeTrigger maps raw trigger.
func NormalizeTrigger(raw string) Trigger {
	switch Trigger(strings.ToUpper(strings.TrimSpace(raw))) {
	case TriggerTaskCompleted:
		return TriggerTaskCompleted
	case TriggerMaintenance:
		return TriggerMaintenance
	case TriggerExternalEvent:
		return TriggerExternalEvent
	default:
		return TriggerSelfScheduled
	}
}

func validateCreate(in CreateInput) error {
	if strings.TrimSpace(in.Title) == "" {
		return fmt.Errorf("title required")
	}
	if in.DueAt.IsZero() {
		return fmt.Errorf("due_at required")
	}
	if NormalizeKind(string(in.Kind)) == IntentRecurring && in.Interval <= 0 {
		return fmt.Errorf("recurring intent requires interval")
	}
	return nil
}
