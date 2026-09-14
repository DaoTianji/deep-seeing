package story

import (
	"path/filepath"
	"time"
)

// This is a public validation verdict, never model chain of thought. Kept in
// the visitor's private diagnostics directory, separate from story memory.
type turnFailure struct {
	RequestID string    `json:"request_id"`
	Revision  int       `json:"revision"`
	Phase     string    `json:"phase"`
	Verdict   string    `json:"public_verdict,omitempty"`
	Created   time.Time `json:"created_at"`
}

func (e *Engine) saveTurnFailure(owner, branch, request string, revision int, phase, verdict string) error {
	if !validID(owner) || !validID(branch) || !validID(request) {
		return ErrNotFound
	}
	runes := []rune(verdict)
	if len(runes) > 400 {
		runes = runes[:400]
	}
	return atomicJSON(filepath.Join(e.Root, owner, "diagnostics", branch+".json"), turnFailure{request, revision, phase, string(runes), time.Now().UTC()})
}
