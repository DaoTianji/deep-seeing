package theater

import (
	"strings"
	"time"

	"deep-seeing/internal/identity"
)

// Mode controls whether the role theater is disabled, observed, or allowed to mutate.
type Mode string

const (
	ModeOff     Mode = "off"
	ModeObserve Mode = "observe"
	ModeAgent   Mode = "agent"
)

type RoleKind string

const (
	RoleCharacter    RoleKind = "character"
	RoleProfessional RoleKind = "professional"
)

type SubjectClass string

const (
	SubjectFictional     SubjectClass = "fictional"
	SubjectDeceased      SubjectClass = "deceased"
	SubjectLivingPublic  SubjectClass = "living_public"
	SubjectLivingPrivate SubjectClass = "living_private"
)

type DefinitionStatus string

const (
	DefinitionDraft      DefinitionStatus = "draft"
	DefinitionValidating DefinitionStatus = "validating"
	DefinitionReady      DefinitionStatus = "ready"
	DefinitionArchived   DefinitionStatus = "archived"
)

type InstanceStatus string

const (
	InstanceIdle   InstanceStatus = "idle"
	InstanceActive InstanceStatus = "active"
	InstancePaused InstanceStatus = "paused"
)

type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionPaused    SessionStatus = "paused"
	SessionCompleted SessionStatus = "completed"
	SessionAborted   SessionStatus = "aborted"
)

type Channel string

const (
	ChannelStage     Channel = "stage"
	ChannelBackstage Channel = "backstage"
)

type MemoryClass string

const (
	MemoryCanonical   MemoryClass = "canonical"
	MemoryInferred    MemoryClass = "inferred"
	MemorySimulated   MemoryClass = "simulated"
	MemoryOperational MemoryClass = "operational"
	MemoryLegacy      MemoryClass = "legacy_roleplay"
)

type ClaimKind string

const (
	ClaimFact         ClaimKind = "fact"
	ClaimBelief       ClaimKind = "belief"
	ClaimVoice        ClaimKind = "voice"
	ClaimRelationship ClaimKind = "relationship"
	ClaimContested    ClaimKind = "contested"
	ClaimUnknown      ClaimKind = "unknown"
)

type TimelineEvent struct {
	When      string   `json:"when"`
	Summary   string   `json:"summary"`
	SourceIDs []string `json:"source_ids,omitempty"`
}

type RoleToolPolicy struct {
	Allowed []string `json:"allowed,omitempty"`
	Denied  []string `json:"denied,omitempty"`
}

// RoleDefinition is the versioned, source-backed role blueprint.
type RoleDefinition struct {
	ID                  string               `json:"id"`
	Scope               identity.TenantScope `json:"scope"`
	DisplayName         string               `json:"display_name"`
	Kind                RoleKind             `json:"kind"`
	SubjectClass        SubjectClass         `json:"subject_class"`
	Description         string               `json:"description,omitempty"`
	Identity            string               `json:"identity,omitempty"`
	Voice               string               `json:"voice,omitempty"`
	KnowledgeCutoff     string               `json:"knowledge_cutoff,omitempty"`
	Timeline            []TimelineEvent      `json:"timeline,omitempty"`
	VariantOfRoleID     string               `json:"variant_of_role_id,omitempty"`
	TargetPeriod        string               `json:"target_period,omitempty"`
	CorpusRoleID        string               `json:"corpus_role_id,omitempty"`
	BlueprintVersion    int64                `json:"blueprint_version,omitempty"`
	InitializationRunID string               `json:"initialization_run_id,omitempty"`
	SourceIDs           []string             `json:"source_ids,omitempty"`
	ToolPolicy          RoleToolPolicy       `json:"tool_policy,omitempty"`
	PrivateSandbox      bool                 `json:"private_sandbox,omitempty"`
	MainInstanceID      string               `json:"main_instance_id,omitempty"`
	Status              DefinitionStatus     `json:"status"`
	Version             int64                `json:"version"`
	Validation          *ValidationReport    `json:"validation,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
}

type RoleDefinitionWrite struct {
	DisplayName     string
	Kind            RoleKind
	SubjectClass    SubjectClass
	Description     string
	Identity        string
	Voice           string
	KnowledgeCutoff string
	VariantOfRoleID string
	TargetPeriod    string
	CorpusRoleID    string
	ToolPolicy      RoleToolPolicy
}

type RoleInstance struct {
	ID                 string            `json:"id"`
	RoleID             string            `json:"role_id"`
	PersonID           string            `json:"person_id"`
	MainWorldlineID    string            `json:"main_worldline_id"`
	CurrentWorldlineID string            `json:"current_worldline_id"`
	Status             InstanceStatus    `json:"status"`
	Scene              string            `json:"scene,omitempty"`
	State              map[string]string `json:"state,omitempty"`
	Relationship       map[string]string `json:"relationship,omitempty"`
	Version            int64             `json:"version"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type RoleWorldline struct {
	ID                string            `json:"id"`
	RoleID            string            `json:"role_id"`
	RoleInstanceID    string            `json:"role_instance_id"`
	ParentWorldlineID string            `json:"parent_worldline_id,omitempty"`
	ForkedFromAction  string            `json:"forked_from_action,omitempty"`
	Label             string            `json:"label"`
	State             map[string]string `json:"state,omitempty"`
	MaskedMemoryIDs   []string          `json:"masked_memory_ids,omitempty"`
	Version           int64             `json:"version"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type RoleSession struct {
	ID             string        `json:"id"`
	RoleID         string        `json:"role_id"`
	RoleInstanceID string        `json:"role_instance_id"`
	WorldlineID    string        `json:"worldline_id"`
	Status         SessionStatus `json:"status"`
	ExitReason     string        `json:"exit_reason,omitempty"`
	StartedAt      time.Time     `json:"started_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	EndedAt        *time.Time    `json:"ended_at,omitempty"`
}

type RoleSource struct {
	ID        string         `json:"id"`
	RoleID    string         `json:"role_id"`
	Title     string         `json:"title"`
	Kind      string         `json:"kind"`
	Audience  SourceAudience `json:"audience"`
	URL       string         `json:"url,omitempty"`
	MimeType  string         `json:"mime_type,omitempty"`
	Path      string         `json:"path,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type SourceAudience string

const (
	SourceActor    SourceAudience = "actor"
	SourceDirector SourceAudience = "director"
)

type RoleClaim struct {
	ID         string    `json:"id"`
	RoleID     string    `json:"role_id"`
	Kind       ClaimKind `json:"kind"`
	Statement  string    `json:"statement"`
	TimeScope  string    `json:"time_scope,omitempty"`
	SourceIDs  []string  `json:"source_ids,omitempty"`
	Confidence float64   `json:"confidence,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type ValidationIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ValidationReport struct {
	Passed      bool              `json:"passed"`
	Issues      []ValidationIssue `json:"issues,omitempty"`
	ValidatedAt time.Time         `json:"validated_at"`
}

type ActionType string

const (
	ActionNoChange              ActionType = "no_change"
	ActionSetScene              ActionType = "set_scene"
	ActionSetRoleState          ActionType = "set_role_state"
	ActionFocusMemory           ActionType = "focus_memory"
	ActionAppendSimulatedMemory ActionType = "append_simulated_memory"
	ActionMaskSimulatedMemory   ActionType = "mask_simulated_memory"
	ActionReviseRoleModel       ActionType = "revise_role_model"
	ActionForkWorldline         ActionType = "fork_worldline"
	ActionPauseRole             ActionType = "pause_role"
	ActionExitRole              ActionType = "exit_role"
)

type ActionStatus string

const (
	ActionExpected ActionStatus = "expected"
	ActionApplied  ActionStatus = "applied"
	ActionReverted ActionStatus = "reverted"
)

type DirectorAction struct {
	ID               string            `json:"id"`
	RoleID           string            `json:"role_id"`
	RoleInstanceID   string            `json:"role_instance_id"`
	RoleSessionID    string            `json:"role_session_id"`
	WorldlineID      string            `json:"worldline_id"`
	TurnID           string            `json:"turn_id,omitempty"`
	Type             ActionType        `json:"type"`
	Status           ActionStatus      `json:"status"`
	ReasonCode       string            `json:"reason_code,omitempty"`
	Before           map[string]string `json:"before,omitempty"`
	After            map[string]string `json:"after,omitempty"`
	ExpectedVersion  int64             `json:"expected_version,omitempty"`
	AppliedVersion   int64             `json:"applied_version,omitempty"`
	RevertsActionID  string            `json:"reverts_action_id,omitempty"`
	TouchesCanonical bool              `json:"touches_canonical,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

type TranscriptMessage struct {
	TurnID    string    `json:"turn_id,omitempty"`
	Channel   Channel   `json:"channel"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func normalizeRoleKind(v RoleKind) RoleKind {
	if v == RoleProfessional {
		return RoleProfessional
	}
	return RoleCharacter
}

func normalizeSubjectClass(v SubjectClass) SubjectClass {
	switch v {
	case SubjectDeceased, SubjectLivingPublic, SubjectLivingPrivate:
		return v
	default:
		return SubjectFictional
	}
}

func isPrivateSubject(v SubjectClass) bool {
	return v == SubjectLivingPrivate
}

func validChannel(v Channel) bool {
	return v == ChannelStage || v == ChannelBackstage
}

func cleanText(v string) string { return strings.TrimSpace(v) }
