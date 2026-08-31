package room

import (
	"encoding/base64"
	"net/http"
	"strings"

	"deep-seeing/internal/theater"
)

func (s *Server) registerRoleInitializationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/role-initializations", s.handleRoleInitializations)
	mux.HandleFunc("POST /api/role-initializations", s.handleStartRoleInitialization)
	mux.HandleFunc("GET /api/role-initializations/{id}", s.handleRoleInitialization)
	mux.HandleFunc("POST /api/role-initializations/{id}/approve-plan", s.handleApproveRolePlan)
	mux.HandleFunc("POST /api/role-initializations/{id}/budget", s.handleGrantRoleBudget)
	mux.HandleFunc("POST /api/role-initializations/{id}/pause", s.handlePauseRoleInitialization)
	mux.HandleFunc("POST /api/role-initializations/{id}/resume", s.handleResumeRoleInitialization)
	mux.HandleFunc("POST /api/role-initializations/{id}/cancel", s.handleCancelRoleInitialization)
	mux.HandleFunc("POST /api/role-initializations/{id}/revision", s.handleRoleBlueprintRevision)
	mux.HandleFunc("POST /api/role-initializations/{id}/approve", s.handleApproveRoleBlueprint)
	mux.HandleFunc("POST /api/role-initializations/{id}/documents", s.handleAddRoleInitializationDocument)
	mux.HandleFunc("GET /api/role-initializations/{id}/corpus", s.handleRoleInitializationCorpus)
	mux.HandleFunc("GET /api/role-corpus/chunks/{id}", s.handleRoleCorpusChunk)
}

func (s *Server) handleRoleInitializations(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.Roles.ListInitializations(r.Context(), r.URL.Query().Get("role_id"), 100)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": items, "mode": s.App.RoleInitMode, "coverage_limited": s.App.RoleArchitect != nil && s.App.RoleArchitect.CoverageLimited})
}

func (s *Server) handleStartRoleInitialization(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DisplayName         string               `json:"display_name"`
		Kind                theater.RoleKind     `json:"kind"`
		SubjectClass        theater.SubjectClass `json:"subject_class"`
		Description         string               `json:"description"`
		Objective           string               `json:"objective"`
		TargetPeriod        string               `json:"target_period"`
		KnowledgeCutoff     string               `json:"knowledge_cutoff"`
		VariantOfRoleID     string               `json:"variant_of_role_id"`
		CorpusRoleID        string               `json:"corpus_role_id"`
		PrivateModelConsent bool                 `json:"private_model_consent"`
	}
	if !decodeRoleJSON(w, r, &in) {
		return
	}
	run, role, err := s.App.RoleArchitect.Start(r.Context(), theater.StartRoleInitializationInput{DisplayName: in.DisplayName, Kind: in.Kind, SubjectClass: in.SubjectClass, Description: in.Description, Objective: in.Objective, TargetPeriod: in.TargetPeriod, KnowledgeCutoff: in.KnowledgeCutoff, VariantOfRoleID: in.VariantOfRoleID, CorpusRoleID: in.CorpusRoleID, PrivateModelConsent: in.PrivateModelConsent})
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"run": run, "role": role})
}

func (s *Server) handleRoleInitialization(w http.ResponseWriter, r *http.Request) {
	run, err := s.App.Roles.GetInitialization(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	payload := map[string]any{"run": run, "mode": s.App.RoleInitMode}
	if role, e := s.App.Roles.GetDefinition(r.Context(), run.RoleID); e == nil {
		payload["role"] = role
	}
	if run.BlueprintID != "" {
		if value, e := s.App.Roles.GetBlueprint(r.Context(), run.BlueprintID); e == nil {
			payload["blueprint"] = value
		}
	}
	if run.CritiqueID != "" {
		if value, e := s.App.Roles.GetCritique(r.Context(), run.CritiqueID); e == nil {
			payload["critique"] = value
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) continueRoleInitialization(id string) {
	s.App.RoleArchitect.ContinueAsync(id)
}

func (s *Server) handleApproveRolePlan(w http.ResponseWriter, r *http.Request) {
	run, err := s.App.Roles.ApproveResearchPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	s.continueRoleInitialization(run.ID)
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run, "started": true})
}
func (s *Server) handleGrantRoleBudget(w http.ResponseWriter, r *http.Request) {
	run, err := s.App.Roles.GrantInitializationBudget(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	s.continueRoleInitialization(run.ID)
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}
func (s *Server) handlePauseRoleInitialization(w http.ResponseWriter, r *http.Request) {
	run, err := s.App.Roles.PauseInitialization(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run})
}
func (s *Server) handleResumeRoleInitialization(w http.ResponseWriter, r *http.Request) {
	run, err := s.App.Roles.ResumeInitialization(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	s.continueRoleInitialization(run.ID)
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}
func (s *Server) handleCancelRoleInitialization(w http.ResponseWriter, r *http.Request) {
	run, err := s.App.Roles.CancelInitialization(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run})
}
func (s *Server) handleRoleBlueprintRevision(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if !decodeRoleJSON(w, r, &in) {
		return
	}
	run, err := s.App.Roles.RequestBlueprintRevision(r.Context(), r.PathValue("id"), in.Reason)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	s.continueRoleInitialization(run.ID)
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}
func (s *Server) handleApproveRoleBlueprint(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WarningAcceptanceReason string `json:"warning_acceptance_reason"`
	}
	_ = decodeOptionalRoleJSON(r, &in)
	run, role, err := s.App.RoleArchitect.ApproveFinal(r.Context(), r.PathValue("id"), in.WarningAcceptanceReason)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	_ = theater.IndexRole(r.Context(), s.App.Graph, s.App.Scope, s.App.Roles, role.ID)
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "role": role})
}

func (s *Server) handleAddRoleInitializationDocument(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title         string                 `json:"title"`
		MimeType      string                 `json:"mime_type"`
		Content       string                 `json:"content"`
		ContentBase64 string                 `json:"content_base64"`
		URL           string                 `json:"url"`
		Audience      theater.SourceAudience `json:"audience"`
		Tier          theater.SourceTier     `json:"tier"`
	}
	if !decodeRoleJSON(w, r, &in) {
		return
	}
	run, err := s.App.Roles.GetInitialization(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	role, err := s.App.Roles.GetDefinition(r.Context(), run.RoleID)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	var body []byte
	if in.ContentBase64 != "" {
		body, err = base64.StdEncoding.DecodeString(in.ContentBase64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid content_base64"})
			return
		}
	} else {
		body = []byte(in.Content)
	}
	if len(body) > theater.MaxRoleDocumentBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "role document exceeds 32 MiB"})
		return
	}
	if strings.EqualFold(in.MimeType, "application/pdf") && len(body) > 0 {
		text, e := theater.ExtractPDFText(body, theater.MaxRoleTextBytes)
		if e != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": e.Error()})
			return
		}
		body = []byte(text)
	}
	if strings.TrimSpace(in.URL) != "" && len(body) == 0 {
		if role.PrivateSandbox {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "private role sources are not fetched from the internet"})
			return
		}
		if _, err = s.App.Roles.ConsumeInitializationRemote(r.Context(), run.ID); err != nil {
			writeRoleError(w, err)
			return
		}
		page, e := s.App.World.ReadWebpage(r.Context(), in.URL)
		if e != nil {
			writeRoleError(w, e)
			return
		}
		body, in.MimeType = []byte(page.Body), page.ContentType
		if strings.TrimSpace(in.Title) == "" {
			in.Title = page.Title
		}
	}
	if len(body) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "document content required"})
		return
	}
	source, err := s.App.Roles.AddSourceWithAudience(r.Context(), role.ID, in.Title, "role_initialization", in.URL, in.MimeType, in.Audience, body)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	document, chunks, duplicate, err := s.App.RoleCorpus.Ingest(r.Context(), theater.CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: source.ID, SourceURL: in.URL, Title: source.Title, MimeType: source.MimeType, Audience: source.Audience, Tier: in.Tier, Text: body})
	if err != nil {
		writeRoleError(w, err)
		return
	}
	assessmentSourceID := source.ID
	if document.SourceID != "" {
		assessmentSourceID = document.SourceID
	}
	assessment := theater.SourceAssessment{SourceID: assessmentSourceID, Tier: in.Tier, Audience: source.Audience, Status: theater.AssessmentAccepted, Reliable: "user_provided", ReasonCode: "user_material"}
	for _, chunk := range chunks {
		assessment.ReadChunkIDs = append(assessment.ReadChunkIDs, chunk.ID)
	}
	run, err = s.App.Roles.SaveSourceAssessment(r.Context(), run.ID, assessment)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"run": run, "source": source, "document": document, "chunks": chunks, "duplicate": duplicate})
}

func (s *Server) handleRoleInitializationCorpus(w http.ResponseWriter, r *http.Request) {
	run, err := s.App.Roles.GetInitialization(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	role, err := s.App.Roles.GetDefinition(r.Context(), run.RoleID)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	query := r.URL.Query().Get("q")
	audience := theater.SourceAudience(r.URL.Query().Get("audience"))
	if strings.TrimSpace(query) != "" {
		cards, e := s.App.RoleCorpus.Search(r.Context(), role.CorpusRoleID, query, audience, 30)
		if e != nil {
			writeRoleError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"chunks": cards})
		return
	}
	documents, err := s.App.RoleCorpus.ListDocuments(r.Context(), role.CorpusRoleID)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": documents})
}
func (s *Server) handleRoleCorpusChunk(w http.ResponseWriter, r *http.Request) {
	chunk, err := s.App.RoleCorpus.ReadChunk(r.Context(), r.PathValue("id"))
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chunk": chunk})
}
