package theater

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/identity"
)

// Store persists role definitions, instances, worldlines, sessions, sources,
// transcripts and director actions. Files are the source of truth; the graph is an index.
type Store struct {
	mu   sync.Mutex
	root string
}

func NewStore(root string) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = filepath.Join("data", "memory", "roles")
	}
	s := &Store{root: root}
	for _, dir := range []string{"definitions", "instances", "worldlines", "sessions", "sources", "materials", "transcripts", "actions", "claims", "initializations", "corpus/documents", "corpus/chunks", "blueprints", "critiques"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) CreateDefinition(_ context.Context, scope identity.TenantScope, w RoleDefinitionWrite) (RoleDefinition, error) {
	if err := scope.Validate(); err != nil {
		return RoleDefinition{}, err
	}
	if cleanText(w.DisplayName) == "" {
		return RoleDefinition{}, fmt.Errorf("role display_name required")
	}
	now := time.Now().UTC()
	d := RoleDefinition{
		ID: "role_" + compactUUID(), Scope: scope, DisplayName: cleanText(w.DisplayName),
		Kind: normalizeRoleKind(w.Kind), SubjectClass: normalizeSubjectClass(w.SubjectClass),
		Description: cleanText(w.Description), Identity: cleanText(w.Identity), Voice: cleanText(w.Voice),
		KnowledgeCutoff: cleanText(w.KnowledgeCutoff), ToolPolicy: normalizeToolPolicy(w.ToolPolicy),
		VariantOfRoleID: cleanText(w.VariantOfRoleID), TargetPeriod: cleanText(w.TargetPeriod), CorpusRoleID: cleanText(w.CorpusRoleID),
		PrivateSandbox: isPrivateSubject(normalizeSubjectClass(w.SubjectClass)),
		Status:         DefinitionDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if d.CorpusRoleID == "" {
		d.CorpusRoleID = d.ID
	}
	if d.PrivateSandbox {
		d.ToolPolicy.Allowed = nil
		d.ToolPolicy.Denied = appendUnique(d.ToolPolicy.Denied, "search_web", "read_webpage", "external_message", "publish", "share")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeJSONAtomic(s.definitionPath(d.ID), d); err != nil {
		return RoleDefinition{}, err
	}
	return d, nil
}

func (s *Store) GetDefinition(_ context.Context, id string) (RoleDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var d RoleDefinition
	if err := readJSON(s.definitionPath(id), &d); err != nil {
		return RoleDefinition{}, err
	}
	return d, nil
}

func (s *Store) ListDefinitions(_ context.Context, scope identity.TenantScope, includeArchived bool) ([]RoleDefinition, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, err := filepath.Glob(filepath.Join(s.root, "definitions", "*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]RoleDefinition, 0)
	for _, path := range paths {
		var d RoleDefinition
		if readJSON(path, &d) != nil || d.Scope != scope || (!includeArchived && d.Status == DefinitionArchived) {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *Store) SaveDefinition(_ context.Context, d RoleDefinition, expectedVersion int64) (RoleDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var current RoleDefinition
	if err := readJSON(s.definitionPath(d.ID), &current); err != nil {
		return RoleDefinition{}, err
	}
	if expectedVersion > 0 && current.Version != expectedVersion {
		return RoleDefinition{}, fmt.Errorf("role version conflict: have %d want %d", current.Version, expectedVersion)
	}
	d.ID, d.Scope, d.CreatedAt = current.ID, current.Scope, current.CreatedAt
	d.Kind = normalizeRoleKind(d.Kind)
	d.SubjectClass = normalizeSubjectClass(d.SubjectClass)
	d.PrivateSandbox = isPrivateSubject(d.SubjectClass)
	if cleanText(d.CorpusRoleID) == "" {
		d.CorpusRoleID = d.ID
	}
	d.Version = current.Version + 1
	d.UpdatedAt = time.Now().UTC()
	if d.PrivateSandbox {
		d.ToolPolicy.Allowed = nil
		d.ToolPolicy.Denied = appendUnique(d.ToolPolicy.Denied, "search_web", "read_webpage", "external_message", "publish", "share")
	}
	if err := writeJSONAtomic(s.definitionPath(d.ID), d); err != nil {
		return RoleDefinition{}, err
	}
	return d, nil
}

func (s *Store) SetSubjectClass(ctx context.Context, roleID string, subject SubjectClass) (RoleDefinition, error) {
	if !validSubjectClass(subject) {
		return RoleDefinition{}, fmt.Errorf("invalid subject_class %q", subject)
	}
	d, err := s.GetDefinition(ctx, roleID)
	if err != nil {
		return RoleDefinition{}, err
	}
	if d.Status == DefinitionReady || d.MainInstanceID != "" {
		return d, fmt.Errorf("subject_class cannot change after a role enters the theater")
	}
	d.SubjectClass = subject
	d.PrivateSandbox = isPrivateSubject(subject)
	d.Validation = nil
	return s.SaveDefinition(ctx, d, d.Version)
}

func (s *Store) SetValidation(ctx context.Context, roleID string, report ValidationReport) (RoleDefinition, error) {
	d, err := s.GetDefinition(ctx, roleID)
	if err != nil {
		return RoleDefinition{}, err
	}
	report.ValidatedAt = time.Now().UTC()
	d.Validation = &report
	d.Status = DefinitionValidating
	return s.SaveDefinition(ctx, d, d.Version)
}

func (s *Store) Publish(ctx context.Context, roleID string) (RoleDefinition, error) {
	if runs, err := s.ListInitializations(ctx, roleID, 1); err == nil && len(runs) > 0 && runs[0].Status != InitCompleted {
		return RoleDefinition{}, fmt.Errorf("role initialization final approval required")
	}
	return s.publishValidated(ctx, roleID)
}

// PublishInitialized is the only path that can publish a role still inside an
// initialization run. It independently rechecks run and Critic state.
func (s *Store) PublishInitialized(ctx context.Context, roleID, runID string) (RoleDefinition, error) {
	run, err := s.GetInitialization(ctx, runID)
	if err != nil {
		return RoleDefinition{}, err
	}
	if run.RoleID != roleID || run.Status != InitAwaitingFinalApproval || run.CritiqueID == "" {
		return RoleDefinition{}, fmt.Errorf("role initialization is not ready for final approval")
	}
	critique, err := s.GetCritique(ctx, run.CritiqueID)
	if err != nil || !critique.Passed || hasUnresolvedHardIssue(critique.Issues) {
		return RoleDefinition{}, fmt.Errorf("critic hard errors block publish")
	}
	return s.publishValidated(ctx, roleID)
}

func (s *Store) publishValidated(ctx context.Context, roleID string) (RoleDefinition, error) {
	d, err := s.GetDefinition(ctx, roleID)
	if err != nil {
		return RoleDefinition{}, err
	}
	if d.Validation == nil || !d.Validation.Passed {
		return RoleDefinition{}, fmt.Errorf("role validation must pass before publish")
	}
	d.Status = DefinitionReady
	return s.SaveDefinition(ctx, d, d.Version)
}

// Enter creates or resumes the role's main instance and starts a new stage session.
func (s *Store) Enter(ctx context.Context, scope identity.TenantScope, roleID string) (RoleDefinition, RoleInstance, RoleSession, error) {
	if err := scope.Validate(); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.activePath()); err == nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, fmt.Errorf("another role session is active or paused")
	}
	var d RoleDefinition
	if err := readJSON(s.definitionPath(roleID), &d); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	if d.Scope != scope {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, fmt.Errorf("role scope mismatch")
	}
	if d.Status != DefinitionReady {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, fmt.Errorf("role is not ready")
	}
	now := time.Now().UTC()
	var inst RoleInstance
	if d.MainInstanceID != "" {
		if err := readJSON(s.instancePath(d.MainInstanceID), &inst); err != nil {
			return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
		}
	} else {
		inst = RoleInstance{
			ID: "rinst_" + compactUUID(), RoleID: d.ID, PersonID: scope.PersonID(), Status: InstanceIdle,
			State: map[string]string{}, Relationship: map[string]string{}, Version: 1, CreatedAt: now, UpdatedAt: now,
		}
		world := RoleWorldline{
			ID: "world_" + compactUUID(), RoleID: d.ID, RoleInstanceID: inst.ID, Label: "main",
			State: map[string]string{}, Version: 1, CreatedAt: now, UpdatedAt: now,
		}
		inst.MainWorldlineID, inst.CurrentWorldlineID = world.ID, world.ID
		if err := writeJSONAtomic(s.worldlinePath(world.ID), world); err != nil {
			return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
		}
		d.MainInstanceID = inst.ID
		d.Version++
		d.UpdatedAt = now
		if err := writeJSONAtomic(s.definitionPath(d.ID), d); err != nil {
			return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
		}
	}
	inst.Status, inst.Version, inst.UpdatedAt = InstanceActive, inst.Version+1, now
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	session := RoleSession{
		ID: "rsess_" + compactUUID(), RoleID: d.ID, RoleInstanceID: inst.ID,
		WorldlineID: inst.CurrentWorldlineID, Status: SessionActive, StartedAt: now, UpdatedAt: now,
	}
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	if err := writeJSONAtomic(s.activePath(), map[string]string{"session_id": session.ID}); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	return d, inst, session, nil
}

func (s *Store) Active(_ context.Context) (RoleDefinition, RoleInstance, RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeLocked()
}

func (s *Store) Pause(_ context.Context, reason string) (RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return RoleSession{}, err
	}
	if session.Status != SessionActive {
		return RoleSession{}, fmt.Errorf("role session is not active")
	}
	now := time.Now().UTC()
	session.Status, session.ExitReason, session.UpdatedAt = SessionPaused, cleanText(reason), now
	inst.Status, inst.Version, inst.UpdatedAt = InstancePaused, inst.Version+1, now
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleSession{}, err
	}
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return RoleSession{}, err
	}
	return session, nil
}

func (s *Store) Resume(_ context.Context) (RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return RoleSession{}, err
	}
	if session.Status != SessionPaused {
		return RoleSession{}, fmt.Errorf("role session is not paused")
	}
	now := time.Now().UTC()
	session.Status, session.ExitReason, session.UpdatedAt = SessionActive, "", now
	inst.Status, inst.Version, inst.UpdatedAt = InstanceActive, inst.Version+1, now
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleSession{}, err
	}
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return RoleSession{}, err
	}
	return session, nil
}

func (s *Store) Exit(_ context.Context, reason string, aborted bool) (RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return RoleSession{}, err
	}
	now := time.Now().UTC()
	if aborted {
		session.Status = SessionAborted
	} else {
		session.Status = SessionCompleted
	}
	session.ExitReason, session.UpdatedAt, session.EndedAt = cleanText(reason), now, &now
	inst.Status, inst.Version, inst.UpdatedAt = InstanceIdle, inst.Version+1, now
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleSession{}, err
	}
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return RoleSession{}, err
	}
	if err := os.Remove(s.activePath()); err != nil && !os.IsNotExist(err) {
		return RoleSession{}, err
	}
	return session, nil
}

// Recover pauses an active role after process restart; no role turn is resumed silently.
func (s *Store) Recover(_ context.Context) (RoleSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		if os.IsNotExist(err) {
			return RoleSession{}, false, nil
		}
		return RoleSession{}, false, err
	}
	if session.Status != SessionActive {
		return session, false, nil
	}
	now := time.Now().UTC()
	session.Status, session.ExitReason, session.UpdatedAt = SessionPaused, "process_recovery", now
	inst.Status, inst.Version, inst.UpdatedAt = InstancePaused, inst.Version+1, now
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleSession{}, false, err
	}
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return RoleSession{}, false, err
	}
	return session, true, nil
}

func (s *Store) ForkWorldline(_ context.Context, actionID, label string) (RoleWorldline, RoleInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inst, session, err := s.activeLocked()
	if err != nil {
		return RoleWorldline{}, RoleInstance{}, err
	}
	var parent RoleWorldline
	if err := readJSON(s.worldlinePath(session.WorldlineID), &parent); err != nil {
		return RoleWorldline{}, RoleInstance{}, err
	}
	now := time.Now().UTC()
	world := RoleWorldline{
		ID: "world_" + compactUUID(), RoleID: parent.RoleID, RoleInstanceID: parent.RoleInstanceID,
		ParentWorldlineID: parent.ID, ForkedFromAction: cleanText(actionID), Label: cleanText(label),
		State: cloneMap(parent.State), MaskedMemoryIDs: append([]string(nil), parent.MaskedMemoryIDs...), Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if world.Label == "" {
		world.Label = "branch"
	}
	inst.CurrentWorldlineID, inst.Version, inst.UpdatedAt = world.ID, inst.Version+1, now
	session.WorldlineID, session.UpdatedAt = world.ID, now
	if err := writeJSONAtomic(s.worldlinePath(world.ID), world); err != nil {
		return RoleWorldline{}, RoleInstance{}, err
	}
	if err := writeJSONAtomic(s.instancePath(inst.ID), inst); err != nil {
		return RoleWorldline{}, RoleInstance{}, err
	}
	if err := writeJSONAtomic(s.sessionPath(session.ID), session); err != nil {
		return RoleWorldline{}, RoleInstance{}, err
	}
	return world, inst, nil
}

func (s *Store) AppendTranscript(_ context.Context, sessionID string, msg TranscriptMessage) error {
	if !validChannel(msg.Channel) {
		return fmt.Errorf("invalid transcript channel")
	}
	msg.Content = cleanText(msg.Content)
	if msg.Content == "" {
		return fmt.Errorf("transcript content required")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return appendJSONLine(s.transcriptPath(sessionID, msg.Channel), msg)
}

func (s *Store) ReadTranscript(_ context.Context, sessionID string, channel Channel, limit int) ([]TranscriptMessage, error) {
	if !validChannel(channel) {
		return nil, fmt.Errorf("invalid transcript channel")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []TranscriptMessage
	if err := readJSONLines(s.transcriptPath(sessionID, channel), &out); err != nil {
		if os.IsNotExist(err) {
			return []TranscriptMessage{}, nil
		}
		return nil, err
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (s *Store) AddSource(_ context.Context, roleID, title, kind, url, mime string, content []byte) (RoleSource, error) {
	return s.AddSourceWithAudience(context.Background(), roleID, title, kind, url, mime, SourceActor, content)
}

func (s *Store) AddSourceWithAudience(_ context.Context, roleID, title, kind, url, mime string, audience SourceAudience, content []byte) (RoleSource, error) {
	if cleanText(title) == "" {
		return RoleSource{}, fmt.Errorf("source title required")
	}
	if audience == "" {
		audience = SourceActor
	}
	if audience != SourceActor && audience != SourceDirector {
		return RoleSource{}, fmt.Errorf("invalid source audience")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var d RoleDefinition
	if err := readJSON(s.definitionPath(roleID), &d); err != nil {
		return RoleSource{}, err
	}
	for _, sourceID := range d.SourceIDs {
		var existing RoleSource
		if readJSON(s.sourcePath(sourceID), &existing) != nil || existing.Audience != audience {
			continue
		}
		canonicalURL := canonicalRoleSourceURL(url)
		sameURL := canonicalURL != "" && canonicalRoleSourceURL(existing.URL) == canonicalURL
		sameBody := false
		if len(content) > 0 && existing.Path != "" {
			if body, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(existing.Path))); err == nil {
				sameBody = bytes.Equal(body, content)
			}
		}
		if sameURL || sameBody {
			return existing, nil
		}
	}
	now := time.Now().UTC()
	src := RoleSource{ID: "rsrc_" + compactUUID(), RoleID: roleID, Title: cleanText(title), Kind: cleanText(kind), Audience: audience, URL: cleanText(url), MimeType: cleanText(mime), CreatedAt: now}
	if len(content) > 0 {
		dir := filepath.Join(s.root, "materials", safeID(roleID))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return RoleSource{}, err
		}
		src.Path = filepath.ToSlash(filepath.Join("materials", safeID(roleID), safeID(src.ID)+".txt"))
		if err := os.WriteFile(filepath.Join(s.root, filepath.FromSlash(src.Path)), content, 0o600); err != nil {
			return RoleSource{}, err
		}
	}
	if err := writeJSONAtomic(s.sourcePath(src.ID), src); err != nil {
		return RoleSource{}, err
	}
	d.SourceIDs = appendUnique(d.SourceIDs, src.ID)
	d.Version++
	d.UpdatedAt = now
	if err := writeJSONAtomic(s.definitionPath(d.ID), d); err != nil {
		return RoleSource{}, err
	}
	return src, nil
}

func (s *Store) GetSource(_ context.Context, id string) (RoleSource, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var src RoleSource
	if err := readJSON(s.sourcePath(id), &src); err != nil {
		return RoleSource{}, nil, err
	}
	if src.Audience == "" {
		src.Audience = SourceActor
	}
	if src.Path == "" {
		return src, nil, nil
	}
	body, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(src.Path)))
	return src, body, err
}

func (s *Store) AddClaim(_ context.Context, claim RoleClaim) (RoleClaim, error) {
	claim.RoleID = cleanText(claim.RoleID)
	claim.Statement = cleanText(claim.Statement)
	if claim.RoleID == "" || claim.Statement == "" {
		return RoleClaim{}, fmt.Errorf("role_id and statement required")
	}
	if claim.ID == "" {
		claim.ID = "rclaim_" + compactUUID()
	}
	if claim.CreatedAt.IsZero() {
		claim.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.definitionPath(claim.RoleID)); err != nil {
		return RoleClaim{}, err
	}
	if err := appendJSONLine(s.claimsPath(claim.RoleID), claim); err != nil {
		return RoleClaim{}, err
	}
	return claim, nil
}

func (s *Store) ListClaims(_ context.Context, roleID string) ([]RoleClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []RoleClaim
	if err := readJSONLines(s.claimsPath(roleID), &out); err != nil {
		if os.IsNotExist(err) {
			return []RoleClaim{}, nil
		}
		return nil, err
	}
	return out, nil
}

func (s *Store) RecordAction(_ context.Context, action DirectorAction) (DirectorAction, error) {
	if !validActionType(action.Type) {
		return DirectorAction{}, fmt.Errorf("invalid director action %q", action.Type)
	}
	if action.ID == "" {
		action.ID = "ract_" + compactUUID()
	}
	if action.Status == "" {
		action.Status = ActionExpected
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if action.RoleSessionID == "" {
		return DirectorAction{}, fmt.Errorf("role_session_id required")
	}
	if err := appendJSONLine(s.actionPath(action.RoleSessionID), action); err != nil {
		return DirectorAction{}, err
	}
	return action, nil
}

func (s *Store) ListActions(_ context.Context, sessionID string) ([]DirectorAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []DirectorAction
	if err := readJSONLines(s.actionPath(sessionID), &out); err != nil {
		if os.IsNotExist(err) {
			return []DirectorAction{}, nil
		}
		return nil, err
	}
	return out, nil
}

func (s *Store) GetInstance(_ context.Context, id string) (RoleInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out RoleInstance
	err := readJSON(s.instancePath(id), &out)
	return out, err
}

func (s *Store) GetWorldline(_ context.Context, id string) (RoleWorldline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out RoleWorldline
	err := readJSON(s.worldlinePath(id), &out)
	return out, err
}

func (s *Store) GetSession(_ context.Context, id string) (RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out RoleSession
	err := readJSON(s.sessionPath(id), &out)
	return out, err
}

func (s *Store) WorldlineAncestry(_ context.Context, id string) ([]RoleWorldline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []RoleWorldline
	seen := map[string]bool{}
	for cleanText(id) != "" && !seen[id] {
		seen[id] = true
		var world RoleWorldline
		if err := readJSON(s.worldlinePath(id), &world); err != nil {
			return nil, err
		}
		out = append(out, world)
		id = world.ParentWorldlineID
	}
	return out, nil
}

func (s *Store) activeLocked() (RoleDefinition, RoleInstance, RoleSession, error) {
	var active map[string]string
	if err := readJSON(s.activePath(), &active); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	var session RoleSession
	if err := readJSON(s.sessionPath(active["session_id"]), &session); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	var inst RoleInstance
	if err := readJSON(s.instancePath(session.RoleInstanceID), &inst); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	var d RoleDefinition
	if err := readJSON(s.definitionPath(session.RoleID), &d); err != nil {
		return RoleDefinition{}, RoleInstance{}, RoleSession{}, err
	}
	return d, inst, session, nil
}

func (s *Store) definitionPath(id string) string {
	return filepath.Join(s.root, "definitions", safeID(id)+".json")
}
func (s *Store) instancePath(id string) string {
	return filepath.Join(s.root, "instances", safeID(id)+".json")
}
func (s *Store) worldlinePath(id string) string {
	return filepath.Join(s.root, "worldlines", safeID(id)+".json")
}
func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.root, "sessions", safeID(id)+".json")
}
func (s *Store) sourcePath(id string) string {
	return filepath.Join(s.root, "sources", safeID(id)+".json")
}
func (s *Store) claimsPath(id string) string {
	return filepath.Join(s.root, "claims", safeID(id)+".jsonl")
}
func (s *Store) actionPath(id string) string {
	return filepath.Join(s.root, "actions", safeID(id)+".jsonl")
}
func (s *Store) activePath() string { return filepath.Join(s.root, "active.json") }
func (s *Store) transcriptPath(id string, channel Channel) string {
	return filepath.Join(s.root, "transcripts", safeID(id), string(channel)+".jsonl")
}

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func appendJSONLine(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	_, err = f.Write(append(raw, '\n'))
	return err
}

func readJSONLines[T any](path string, out *[]T) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var item T
		if json.Unmarshal(scanner.Bytes(), &item) == nil {
			*out = append(*out, item)
		}
	}
	return scanner.Err()
}

func safeID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "missing"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, raw)
}

func compactUUID() string { return strings.ReplaceAll(uuid.NewString(), "-", "") }

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func appendUnique(existing []string, values ...string) []string {
	out := append([]string(nil), existing...)
	seen := map[string]bool{}
	for _, item := range out {
		seen[item] = true
	}
	for _, item := range values {
		item = cleanText(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func normalizeToolPolicy(in RoleToolPolicy) RoleToolPolicy {
	return RoleToolPolicy{Allowed: appendUnique(nil, in.Allowed...), Denied: appendUnique(nil, in.Denied...)}
}

func validActionType(v ActionType) bool {
	switch v {
	case ActionNoChange, ActionSetScene, ActionSetRoleState, ActionFocusMemory,
		ActionAppendSimulatedMemory, ActionMaskSimulatedMemory, ActionReviseRoleModel,
		ActionForkWorldline, ActionPauseRole, ActionExitRole:
		return true
	default:
		return false
	}
}
