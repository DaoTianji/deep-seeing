package room

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"deep-seeing/internal/theater"
)

const maxRoleMaterialBytes = 2 << 20

func (s *Server) registerRoleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/roles", s.handleRoles)
	mux.HandleFunc("POST /api/roles", s.handleCreateRole)
	mux.HandleFunc("GET /api/roles/{id}", s.handleRole)
	mux.HandleFunc("POST /api/roles/{id}/sources", s.handleAddRoleSource)
	mux.HandleFunc("POST /api/roles/{id}/compile", s.handleCompileRole)
	mux.HandleFunc("POST /api/roles/{id}/publish", s.handlePublishRole)
	mux.HandleFunc("POST /api/roles/{id}/enter", s.handleEnterRole)
	mux.HandleFunc("GET /api/role/active", s.handleActiveRole)
	mux.HandleFunc("POST /api/role/pause", s.handlePauseRole)
	mux.HandleFunc("POST /api/role/resume", s.handleResumeRole)
	mux.HandleFunc("POST /api/role/exit", s.handleExitRole)
	mux.HandleFunc("POST /api/role/fork", s.handleForkRole)
	mux.HandleFunc("GET /api/role/transcripts", s.handleRoleTranscripts)
	mux.HandleFunc("GET /api/role/actions", s.handleRoleActions)
	mux.HandleFunc("POST /api/director-actions/{id}/revert", s.handleRevertDirectorAction)
}

func (s *Server) handleRoles(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Roles.ListDefinitions(r.Context(), s.App.Scope, r.URL.Query().Get("archived") == "1")
	if err != nil {
		writeError(w, err)
		return
	}
	var active any
	if d, inst, session, activeErr := s.App.Roles.Active(r.Context()); activeErr == nil {
		active = map[string]any{"definition": d, "instance": inst, "session": session}
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": items, "active": active, "mode": s.App.RoleMode})
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName     string               `json:"display_name"`
		Kind            theater.RoleKind     `json:"kind"`
		SubjectClass    theater.SubjectClass `json:"subject_class"`
		Description     string               `json:"description"`
		Identity        string               `json:"identity"`
		Voice           string               `json:"voice"`
		KnowledgeCutoff string               `json:"knowledge_cutoff"`
		AllowedTools    []string             `json:"allowed_tools"`
	}
	if !decodeRoleJSON(w, r, &input) {
		return
	}
	d, err := s.App.Roles.CreateDefinition(r.Context(), s.App.Scope, theater.RoleDefinitionWrite{
		DisplayName: input.DisplayName, Kind: input.Kind, SubjectClass: input.SubjectClass,
		Description: input.Description, Identity: input.Identity, Voice: input.Voice,
		KnowledgeCutoff: input.KnowledgeCutoff,
		ToolPolicy:      theater.RoleToolPolicy{Allowed: input.AllowedTools},
	})
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, d.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"role": d})
}

func (s *Server) handleRole(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	d, err := s.App.Roles.GetDefinition(r.Context(), id)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	sources, err := s.App.Roles.ListSources(r.Context(), id)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	claims, err := s.App.Roles.ListClaims(r.Context(), id)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	sessions, err := s.App.Roles.ListSessions(r.Context(), id, 20)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	var instance any
	var worldlines any
	if d.MainInstanceID != "" {
		inst, instErr := s.App.Roles.GetInstance(r.Context(), d.MainInstanceID)
		if instErr == nil {
			instance = inst
			worldlines, _ = s.App.Roles.ListWorldlines(r.Context(), inst.ID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"role": d, "sources": sources, "claims": claims, "instance": instance,
		"worldlines": worldlines, "sessions": sessions,
	})
}

func (s *Server) handleAddRoleSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title         string                 `json:"title"`
		Kind          string                 `json:"kind"`
		URL           string                 `json:"url"`
		MimeType      string                 `json:"mime_type"`
		Content       string                 `json:"content"`
		ContentBase64 string                 `json:"content_base64"`
		Audience      theater.SourceAudience `json:"audience"`
	}
	if !decodeRoleJSON(w, r, &input) {
		return
	}
	var body []byte
	if input.ContentBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(input.ContentBase64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid content_base64"})
			return
		}
		body = decoded
	} else {
		body = []byte(input.Content)
	}
	if len(body) > maxRoleMaterialBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "role material exceeds 2 MiB"})
		return
	}
	if strings.EqualFold(input.MimeType, "application/pdf") && len(body) > 0 {
		text, extractErr := theater.ExtractPDFText(body, 4<<20)
		if extractErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": extractErr.Error()})
			return
		}
		body = []byte(text)
	}
	if strings.TrimSpace(input.URL) != "" && len(body) == 0 {
		definition, getErr := s.App.Roles.GetDefinition(r.Context(), r.PathValue("id"))
		if getErr != nil {
			writeRoleError(w, getErr)
			return
		}
		if definition.PrivateSandbox {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "private role sources are not fetched from the internet"})
			return
		}
		fetched, fetchErr := s.App.World.ReadWebpage(r.Context(), input.URL)
		if fetchErr != nil {
			writeRoleError(w, fetchErr)
			return
		}
		body = []byte(fetched.Body)
		input.MimeType = fetched.ContentType
		if strings.TrimSpace(input.Title) == "" {
			input.Title = fetched.Title
		}
	}
	source, err := s.App.Roles.AddSourceWithAudience(r.Context(), r.PathValue("id"), input.Title, input.Kind, input.URL, input.MimeType, input.Audience, body)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, r.PathValue("id"))
	writeJSON(w, http.StatusCreated, map[string]any{"source": source})
}

func (s *Server) handleCompileRole(w http.ResponseWriter, r *http.Request) {
	var result any
	err := s.queue().RunCognitive(r.Context(), "role_compile", func(ctx context.Context) error {
		compiled, compileErr := s.App.RoleCompiler.Compile(ctx, r.PathValue("id"))
		if compileErr != nil {
			return compileErr
		}
		result = compiled
		_ = theater.IndexRole(ctx, s.App.Graph, s.App.Scope, s.App.Roles, r.PathValue("id"))
		return nil
	})
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePublishRole(w http.ResponseWriter, r *http.Request) {
	d, err := s.App.Roles.Publish(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, d.ID)
	writeJSON(w, http.StatusOK, map[string]any{"role": d})
}

func (s *Server) handleEnterRole(w http.ResponseWriter, r *http.Request) {
	if s.App.RoleMode == theater.ModeOff {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "ROLE_MODE is off"})
		return
	}
	var d theater.RoleDefinition
	var inst theater.RoleInstance
	var session theater.RoleSession
	err := s.queue().RunCognitive(r.Context(), "role_enter", func(ctx context.Context) error {
		var enterErr error
		d, inst, session, enterErr = s.App.Roles.Enter(ctx, s.App.Scope, r.PathValue("id"))
		return enterErr
	})
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, d.ID)
	writeJSON(w, http.StatusOK, map[string]any{"definition": d, "instance": inst, "session": session})
}

func (s *Server) handleActiveRole(w http.ResponseWriter, r *http.Request) {
	d, inst, session, err := s.App.Roles.Active(r.Context())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusOK, map[string]any{"active": false, "mode": s.App.RoleMode})
			return
		}
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active": true, "mode": s.App.RoleMode, "definition": d, "instance": inst, "session": session})
}

func (s *Server) handlePauseRole(w http.ResponseWriter, r *http.Request) {
	session, err := s.App.Roles.Pause(r.Context(), "user")
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, session.RoleID)
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) handleResumeRole(w http.ResponseWriter, r *http.Request) {
	session, err := s.App.Roles.Resume(r.Context())
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, session.RoleID)
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) handleExitRole(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Reason string `json:"reason"`
	}
	_ = decodeOptionalRoleJSON(r, &input)
	session, err := s.App.Roles.Exit(r.Context(), nonemptyRoom(input.Reason, "user_exit"), false)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, session.RoleID)
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) handleForkRole(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Label string `json:"label"`
	}
	if !decodeRoleJSON(w, r, &input) {
		return
	}
	world, inst, err := s.App.Roles.ForkWorldline(r.Context(), "", input.Label)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, world.RoleID)
	writeJSON(w, http.StatusOK, map[string]any{"worldline": world, "instance": inst})
}

func (s *Server) handleRoleTranscripts(w http.ResponseWriter, r *http.Request) {
	_, _, session, err := s.App.Roles.Active(r.Context())
	if err != nil {
		writeRoleError(w, err)
		return
	}
	channel := theater.Channel(r.URL.Query().Get("channel"))
	if channel == "" {
		channel = theater.ChannelStage
	}
	items, err := s.App.Roles.ReadTranscript(r.Context(), session.ID, channel, queryLimit(r, 200))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session_id": session.ID, "channel": channel, "messages": items})
}

func (s *Server) handleRoleActions(w http.ResponseWriter, r *http.Request) {
	_, _, session, err := s.App.Roles.Active(r.Context())
	if err != nil {
		writeRoleError(w, err)
		return
	}
	items, err := s.App.Roles.ListActions(r.Context(), session.ID)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session_id": session.ID, "actions": items})
}

func (s *Server) handleRevertDirectorAction(w http.ResponseWriter, r *http.Request) {
	if s.App.Theater == nil || s.App.Theater.Reviewer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "director unavailable"})
		return
	}
	_, _, session, err := s.App.Roles.Active(r.Context())
	if err != nil {
		writeRoleError(w, err)
		return
	}
	action, err := s.App.Theater.Reviewer.Revert(r.Context(), session.ID, r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": action})
}

func decodeRoleJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRoleMaterialBytes+65536))
	if err := decoder.Decode(out); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return false
	}
	return true
}

func decodeOptionalRoleJSON(r *http.Request, out any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 65536)).Decode(out)
}

func writeRoleError(w http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "role resource not found"})
		return
	}
	message := err.Error()
	status := http.StatusBadRequest
	if strings.Contains(message, "version conflict") || strings.Contains(message, "another role") || strings.Contains(message, "session changed") {
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]any{"error": message})
}

func nonemptyRoom(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
