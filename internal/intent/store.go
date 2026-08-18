package intent

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store persists intents and wake jobs in SQLite runtime.db.
type Store struct {
	db *sql.DB
}

// OpenStore opens or creates runtime.db under dir (default data/runtime).
func OpenStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "runtime")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "runtime.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS intents (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT,
  status TEXT NOT NULL,
  trigger TEXT NOT NULL,
  due_at TEXT NOT NULL,
  interval_seconds INTEGER,
  stale_after_days INTEGER,
  allow_outbound INTEGER NOT NULL DEFAULT 0,
  attempt INTEGER NOT NULL DEFAULT 0,
  last_wake_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_intents_due ON intents(status, due_at);
CREATE TABLE IF NOT EXISTS wake_jobs (
  id TEXT PRIMARY KEY,
  intent_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  trigger TEXT NOT NULL,
  scheduled_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  status TEXT NOT NULL,
  decision TEXT,
  result TEXT,
  session_id TEXT,
  notes TEXT,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_wake_intent ON wake_jobs(intent_id, created_at);
`)
	return err
}

// Create inserts a new active intent.
func (s *Store) Create(_ context.Context, in CreateInput) (Intent, error) {
	if err := validateCreate(in); err != nil {
		return Intent{}, err
	}
	now := time.Now().UTC()
	it := Intent{
		ID:      "int_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16],
		AgentID: strings.TrimSpace(in.AgentID), Kind: NormalizeKind(string(in.Kind)),
		Title: strings.TrimSpace(in.Title), Body: strings.TrimSpace(in.Body),
		Status: StatusActive, Trigger: NormalizeTrigger(string(in.Trigger)),
		DueAt: in.DueAt.UTC(), Interval: in.Interval, StaleAfterDays: in.StaleAfterDays,
		AllowOutbound: in.AllowOutbound, CreatedAt: now, UpdatedAt: now,
	}
	if it.AgentID == "" {
		it.AgentID = "local"
	}
	if it.StaleAfterDays <= 0 {
		it.StaleAfterDays = 7
	}
	_, err := s.db.Exec(`
INSERT INTO intents(
  id, agent_id, kind, title, body, status, trigger, due_at, interval_seconds,
  stale_after_days, allow_outbound, attempt, last_wake_at, created_at, updated_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		it.ID, it.AgentID, string(it.Kind), it.Title, it.Body, string(it.Status), string(it.Trigger),
		it.DueAt.Format(time.RFC3339), int64(it.Interval/time.Second), it.StaleAfterDays,
		boolInt(it.AllowOutbound), it.Attempt, nullTime(it.LastWakeAt),
		it.CreatedAt.Format(time.RFC3339), it.UpdatedAt.Format(time.RFC3339),
	)
	return it, err
}

// Get loads one intent.
func (s *Store) Get(_ context.Context, id string) (Intent, error) {
	id = strings.TrimSpace(id)
	row := s.db.QueryRow(`
SELECT id, agent_id, kind, title, body, status, trigger, due_at, interval_seconds,
       stale_after_days, allow_outbound, attempt, last_wake_at, created_at, updated_at
FROM intents WHERE id = ?`, id)
	return scanIntent(row)
}

// ListActive returns active intents for an agent (optional filter).
func (s *Store) ListActive(_ context.Context, agentID string, limit int) ([]Intent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
SELECT id, agent_id, kind, title, body, status, trigger, due_at, interval_seconds,
       stale_after_days, allow_outbound, attempt, last_wake_at, created_at, updated_at
FROM intents WHERE status = ? AND (? = '' OR agent_id = ?)
ORDER BY due_at ASC LIMIT ?`, string(StatusActive), agentID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntents(rows)
}

// ListRecent returns intents of every status, newest updates first.
func (s *Store) ListRecent(_ context.Context, agentID string, limit int) ([]Intent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
SELECT id, agent_id, kind, title, body, status, trigger, due_at, interval_seconds,
       stale_after_days, allow_outbound, attempt, last_wake_at, created_at, updated_at
FROM intents WHERE (? = '' OR agent_id = ?)
ORDER BY updated_at DESC LIMIT ?`, agentID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntents(rows)
}

// ListDue returns active intents with due_at <= now.
func (s *Store) ListDue(_ context.Context, agentID string, now time.Time, limit int) ([]Intent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
SELECT id, agent_id, kind, title, body, status, trigger, due_at, interval_seconds,
       stale_after_days, allow_outbound, attempt, last_wake_at, created_at, updated_at
FROM intents
WHERE status = ? AND due_at <= ? AND (? = '' OR agent_id = ?)
ORDER BY due_at ASC LIMIT ?`,
		string(StatusActive), now.UTC().Format(time.RFC3339), agentID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntents(rows)
}

// SetStatus updates intent status.
func (s *Store) SetStatus(_ context.Context, id string, status Status) (Intent, error) {
	status = NormalizeStatus(string(status))
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE intents SET status = ?, updated_at = ? WHERE id = ?`, string(status), now, id)
	if err != nil {
		return Intent{}, err
	}
	return s.Get(context.Background(), id)
}

// MarkWoke updates attempt, last_wake, and next due for recurring.
func (s *Store) MarkWoke(_ context.Context, id string, when time.Time, nextDue time.Time, status Status) (Intent, error) {
	if when.IsZero() {
		when = time.Now().UTC()
	}
	now := when.UTC()
	it, err := s.Get(context.Background(), id)
	if err != nil {
		return Intent{}, err
	}
	it.Attempt++
	it.LastWakeAt = now
	it.UpdatedAt = now
	if status != "" {
		it.Status = NormalizeStatus(string(status))
	}
	if !nextDue.IsZero() {
		it.DueAt = nextDue.UTC()
	}
	_, err = s.db.Exec(`
UPDATE intents SET attempt = ?, last_wake_at = ?, due_at = ?, status = ?, updated_at = ?
WHERE id = ?`,
		it.Attempt, now.Format(time.RFC3339), it.DueAt.Format(time.RFC3339),
		string(it.Status), now.Format(time.RFC3339), id,
	)
	if err != nil {
		return Intent{}, err
	}
	return it, nil
}

// InsertWakeJob records a wake job.
func (s *Store) InsertWakeJob(_ context.Context, job WakeJob) (WakeJob, error) {
	if job.ID == "" {
		job.ID = "wake_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	if job.Status == "" {
		job.Status = "pending"
	}
	_, err := s.db.Exec(`
INSERT INTO wake_jobs(
  id, intent_id, agent_id, trigger, scheduled_at, started_at, finished_at,
  status, decision, result, session_id, notes, created_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		job.ID, job.IntentID, job.AgentID, string(job.Trigger),
		job.ScheduledAt.UTC().Format(time.RFC3339), nullTime(job.StartedAt), nullTime(job.FinishedAt),
		job.Status, job.Decision, job.Result, job.SessionID, job.Notes,
		job.CreatedAt.UTC().Format(time.RFC3339),
	)
	return job, err
}

// FinishWakeJob updates terminal fields.
func (s *Store) FinishWakeJob(_ context.Context, id, status, decision, result, notes string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
UPDATE wake_jobs SET status = ?, decision = ?, result = ?, notes = ?, finished_at = ?
WHERE id = ?`, status, decision, result, notes, now, id)
	return err
}

// ListWakeJobs returns recent wakes for an intent or agent.
func (s *Store) ListWakeJobs(_ context.Context, intentID string, limit int) ([]WakeJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
SELECT id, intent_id, agent_id, trigger, scheduled_at, started_at, finished_at,
       status, decision, result, session_id, notes, created_at
FROM wake_jobs WHERE (? = '' OR intent_id = ?)
ORDER BY created_at DESC LIMIT ?`, intentID, intentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WakeJob
	for rows.Next() {
		j, err := scanWake(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanIntent(row scannable) (Intent, error) {
	var it Intent
	var kind, status, trigger, dueAt, created, updated string
	var intervalSec sql.NullInt64
	var stale sql.NullInt64
	var outbound int
	var lastWake sql.NullString
	var body sql.NullString
	err := row.Scan(
		&it.ID, &it.AgentID, &kind, &it.Title, &body, &status, &trigger, &dueAt,
		&intervalSec, &stale, &outbound, &it.Attempt, &lastWake, &created, &updated,
	)
	if err != nil {
		return Intent{}, err
	}
	it.Kind = NormalizeKind(kind)
	it.Status = NormalizeStatus(status)
	it.Trigger = NormalizeTrigger(trigger)
	it.Body = body.String
	it.AllowOutbound = outbound != 0
	if intervalSec.Valid {
		it.Interval = time.Duration(intervalSec.Int64) * time.Second
	}
	if stale.Valid {
		it.StaleAfterDays = int(stale.Int64)
	}
	it.DueAt, _ = time.Parse(time.RFC3339, dueAt)
	it.CreatedAt, _ = time.Parse(time.RFC3339, created)
	it.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	if lastWake.Valid {
		it.LastWakeAt, _ = time.Parse(time.RFC3339, lastWake.String)
	}
	return it, nil
}

func scanIntents(rows *sql.Rows) ([]Intent, error) {
	var out []Intent
	for rows.Next() {
		it, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func scanWake(row scannable) (WakeJob, error) {
	var j WakeJob
	var trigger, scheduled, created string
	var started, finished, decision, result, session, notes sql.NullString
	err := row.Scan(
		&j.ID, &j.IntentID, &j.AgentID, &trigger, &scheduled, &started, &finished,
		&j.Status, &decision, &result, &session, &notes, &created,
	)
	if err != nil {
		return WakeJob{}, err
	}
	j.Trigger = NormalizeTrigger(trigger)
	j.ScheduledAt, _ = time.Parse(time.RFC3339, scheduled)
	j.CreatedAt, _ = time.Parse(time.RFC3339, created)
	j.Decision = decision.String
	j.Result = result.String
	j.SessionID = session.String
	j.Notes = notes.String
	if started.Valid {
		j.StartedAt, _ = time.Parse(time.RFC3339, started.String)
	}
	if finished.Valid {
		j.FinishedAt, _ = time.Parse(time.RFC3339, finished.String)
	}
	return j, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// EstimateMissedTicks approximates missed recurring ticks between due and now.
func EstimateMissedTicks(it Intent, now time.Time) int {
	if it.Kind != IntentRecurring || it.Interval <= 0 || !now.After(it.DueAt) {
		if now.After(it.DueAt) {
			return 1
		}
		return 0
	}
	n := int(now.Sub(it.DueAt)/it.Interval) + 1
	if n < 1 {
		return 1
	}
	if n > 1000 {
		return 1000
	}
	return n
}
