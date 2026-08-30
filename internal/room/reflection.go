package room

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"deep-seeing/internal/memory"
)

const reflectionIdleDelay = 20 * time.Minute

func (s *Server) handleReflections(w http.ResponseWriter, r *http.Request) {
	includeDone := r.URL.Query().Get("include_done") == "1"
	items, err := s.App.Reflections.List(r.Context(), s.App.Scope, s.App.Scope.PersonID(), includeDone, queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reflections": items})
}

func (s *Server) handleReflectionRuns(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Reflections.ListRuns(queryLimit(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": items})
}

func (s *Server) handleReflectionLive(w http.ResponseWriter, _ *http.Request) {
	if s.App.Reflection == nil || s.App.Reflection.Live == nil {
		writeJSON(w, http.StatusOK, memory.ReflectionLiveState{})
		return
	}
	writeJSON(w, http.StatusOK, s.App.Reflection.Live.Snapshot())
}

func (s *Server) handleGenerativeDream(w http.ResponseWriter, r *http.Request) {
	var result any
	err := s.queue().RunCognitive(r.Context(), "generative_dream", func(ctx context.Context) error {
		res, runErr := s.App.Generative.Run(ctx, s.App.Scope, s.App.SessionID, memory.ReflectionTriggerManual)
		if runErr != nil {
			return runErr
		}
		result = res
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRevertMutation(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if strings.TrimSpace(payload.Reason) == "" {
		payload.Reason = "user requested rollback"
	}
	var result any
	err := s.queue().RunCognitive(r.Context(), "reflection_rollback", func(ctx context.Context) error {
		mutation, revertErr := s.App.Dreamer.RevertMutation(ctx, s.App.Scope, r.PathValue("id"), payload.Reason)
		if revertErr != nil {
			return revertErr
		}
		result = mutation
		if s.App.Service != nil {
			s.App.Service.InvalidateNorm()
		}
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mutation": result})
}

func (s *Server) scheduleIdleReview() {
	if s == nil || s.App == nil || s.App.ReflectionMode == memory.ReflectionModeLegacy {
		return
	}
	s.idleMu.Lock()
	defer s.idleMu.Unlock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.idleTimer = time.AfterFunc(reflectionIdleDelay, s.runIdleReview)
}

func (s *Server) runIdleReview() {
	if s == nil || s.App == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	history, err := s.App.STM.Get(s.App.SessionID)
	if err != nil {
		log.Printf("idle review history: %v", err)
		return
	}
	var review memory.ReviewResult
	err = s.queue().RunCognitive(ctx, "idle_review", func(runCtx context.Context) error {
		result, runErr := s.App.Reviewer.Run(runCtx, s.App.Scope, s.App.SessionID, history)
		review = result
		return runErr
	})
	if err != nil {
		log.Printf("idle review: %v", err)
		return
	}
	dirty, dirtyErr := s.App.Reflections.IsDirty(s.App.Scope.PersonID())
	if dirtyErr != nil {
		log.Printf("reflection dirty state: %v", dirtyErr)
	}
	if (len(review.ReflectionSeedIDs) == 0 && !dirty.Dirty) || !s.claimAutoReflectionBudget(time.Now().UTC()) {
		return
	}
	var reflection memory.ReflectionRun
	err = s.queue().RunCognitive(ctx, "auto_reflection", func(runCtx context.Context) error {
		result, runErr := s.App.Reflection.Run(runCtx, s.App.Scope, s.App.SessionID, memory.ReflectionTriggerDirty)
		reflection = result
		return runErr
	})
	if err != nil {
		log.Printf("auto reflection: %v", err)
		return
	}
	if reflectionOpenedTension(reflection) {
		if err := s.queue().RunCognitive(ctx, "tension_dream", func(runCtx context.Context) error {
			_, runErr := s.App.Generative.Run(runCtx, s.App.Scope, s.App.SessionID, memory.ReflectionTriggerTension)
			return runErr
		}); err != nil {
			log.Printf("tension dream: %v", err)
		}
	}
}

func (s *Server) claimAutoReflectionBudget(now time.Time) bool {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	day := now.UTC().Format("2006-01-02")
	if s.autoReflectionDay != day {
		s.autoReflectionDay, s.autoReflectionCount = day, 0
	}
	if !s.lastAutoReflection.IsZero() && now.Sub(s.lastAutoReflection) < 6*time.Hour {
		return false
	}
	if s.autoReflectionCount >= 2 {
		return false
	}
	s.lastAutoReflection = now
	s.autoReflectionCount++
	return true
}

func reflectionOpenedTension(run memory.ReflectionRun) bool {
	for _, decision := range run.Decisions {
		if decision.Action == memory.ReflectionOpenTension {
			return true
		}
	}
	return false
}
