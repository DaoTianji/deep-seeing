package observe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TurnTrace is a structured external trajectory for one conversational turn.
// Not chain-of-thought — recall, tools, writes, errors only.
type TurnTrace struct {
	Timestamp     time.Time `json:"timestamp"`
	SessionID     string    `json:"session_id"`
	AgentID       string    `json:"agent_id,omitempty"`
	PersonID      string    `json:"person_id,omitempty"`
	ModelVersion  string    `json:"model_version,omitempty"`
	RuntimeVer    string    `json:"toolset_version,omitempty"`
	UserText      string    `json:"user_text,omitempty"`
	RecallIDs     []string  `json:"recall_ids,omitempty"`
	ToolStarts    []string  `json:"tool_starts,omitempty"`
	MemoryWrites  []string  `json:"memory_writes,omitempty"`
	Proposals     []string  `json:"proposals,omitempty"`
	Errors        []string  `json:"errors,omitempty"`
	AnswerPreview string    `json:"answer_preview,omitempty"`
}

// Journal appends JSONL turn traces.
type Journal struct {
	mu  sync.Mutex
	dir string
}

// NewJournal opens or creates a traces directory.
func NewJournal(dir string) (*Journal, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("data", "memory", "traces")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Journal{dir: dir}, nil
}

// Root returns the traces directory.
func (j *Journal) Root() string {
	if j == nil {
		return ""
	}
	return j.dir
}

// Append writes one turn trace.
func (j *Journal) Append(t TurnTrace) error {
	if j == nil {
		return nil
	}
	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now().UTC()
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	path := filepath.Join(j.dir, t.Timestamp.UTC().Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(raw, '\n'))
	return err
}

// ListRecent returns recent traces across day files, newest first.
func (j *Journal) ListRecent(limit int) ([]TurnTrace, error) {
	if j == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	entries, err := os.ReadDir(j.dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			files = append(files, entry.Name())
		}
	}
	var out []TurnTrace
	for i := len(files) - 1; i >= 0 && len(out) < limit; i-- {
		data, err := os.ReadFile(filepath.Join(j.dir, files[i]))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for k := len(lines) - 1; k >= 0 && len(out) < limit; k-- {
			var trace TurnTrace
			if err := json.Unmarshal([]byte(lines[k]), &trace); err == nil {
				out = append(out, trace)
			}
		}
	}
	return out, nil
}

// Preview truncates answer for the journal.
func Preview(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 {
		n = 200
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
