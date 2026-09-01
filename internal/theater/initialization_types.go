package theater

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type InitializationStatus string

const (
	InitDraft                 InitializationStatus = "draft"
	InitPlanning              InitializationStatus = "planning"
	InitAwaitingPlanApproval  InitializationStatus = "awaiting_plan_approval"
	InitCollecting            InitializationStatus = "collecting"
	InitAnalyzing             InitializationStatus = "analyzing"
	InitCompiling             InitializationStatus = "compiling"
	InitBlueprinting          InitializationStatus = "blueprinting"
	InitCritiquing            InitializationStatus = "critiquing"
	InitAwaitingFinalApproval InitializationStatus = "awaiting_final_approval"
	InitCompleted             InitializationStatus = "completed"
	InitPaused                InitializationStatus = "paused"
	InitNeedsBudget           InitializationStatus = "needs_budget"
	InitFailed                InitializationStatus = "failed"
	InitCancelled             InitializationStatus = "cancelled"
)

type ResearchQuestion struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Topics   []string `json:"topics,omitempty"`
	Priority string   `json:"priority,omitempty"`
}

type RoleResearchPlan struct {
	TargetPeriod       string             `json:"target_period"`
	KnowledgeCutoff    string             `json:"knowledge_cutoff,omitempty"`
	Questions          []ResearchQuestion `json:"questions"`
	RequiredCoverage   []string           `json:"required_coverage,omitempty"`
	PreferredSources   []string           `json:"preferred_sources,omitempty"`
	CompletionCriteria []string           `json:"completion_criteria,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	ApprovedAt         *time.Time         `json:"approved_at,omitempty"`
	Planner            string             `json:"planner,omitempty"`
	Warning            string             `json:"warning,omitempty"`
}

func (p *RoleResearchPlan) UnmarshalJSON(raw []byte) error {
	var wire struct {
		TargetPeriod       json.RawMessage    `json:"target_period"`
		KnowledgeCutoff    json.RawMessage    `json:"knowledge_cutoff"`
		Questions          []ResearchQuestion `json:"questions"`
		RequiredCoverage   []string           `json:"required_coverage"`
		PreferredSources   []string           `json:"preferred_sources"`
		CompletionCriteria []string           `json:"completion_criteria"`
		CreatedAt          time.Time          `json:"created_at"`
		ApprovedAt         *time.Time         `json:"approved_at"`
		Planner            string             `json:"planner"`
		Warning            string             `json:"warning"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	p.TargetPeriod = flexiblePlanText(wire.TargetPeriod)
	p.KnowledgeCutoff = flexiblePlanText(wire.KnowledgeCutoff)
	p.Questions, p.RequiredCoverage, p.PreferredSources = wire.Questions, wire.RequiredCoverage, wire.PreferredSources
	p.CompletionCriteria, p.CreatedAt, p.ApprovedAt = wire.CompletionCriteria, wire.CreatedAt, wire.ApprovedAt
	p.Planner, p.Warning = wire.Planner, wire.Warning
	return nil
}

func flexiblePlanText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var items []string
	if json.Unmarshal(raw, &items) == nil {
		return strings.Join(items, "；")
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		for _, key := range []string{"label", "name", "period", "description", "value"} {
			if value := strings.TrimSpace(fmt.Sprint(object[key])); value != "" && value != "<nil>" {
				return value
			}
		}
		start, end := strings.TrimSpace(fmt.Sprint(object["start"])), strings.TrimSpace(fmt.Sprint(object["end"]))
		if start != "" && start != "<nil>" {
			if end != "" && end != "<nil>" {
				return start + "—" + end
			}
			return start
		}
	}
	return ""
}

type SourceAssessmentStatus string

const (
	AssessmentCandidate SourceAssessmentStatus = "candidate"
	AssessmentRead      SourceAssessmentStatus = "read"
	AssessmentAccepted  SourceAssessmentStatus = "accepted"
	AssessmentDismissed SourceAssessmentStatus = "dismissed"
)

type SourceTier string

const (
	SourcePrimary      SourceTier = "primary"
	SourceContemporary SourceTier = "contemporary"
	SourceBiography    SourceTier = "biography"
	SourceScholarship  SourceTier = "scholarship"
	SourcePosthumous   SourceTier = "posthumous"
	SourceGenerated    SourceTier = "generated"
)

type SourceAssessment struct {
	SourceID     string                 `json:"source_id"`
	Tier         SourceTier             `json:"tier"`
	Audience     SourceAudience         `json:"audience"`
	Status       SourceAssessmentStatus `json:"status"`
	Reliable     string                 `json:"reliable,omitempty"`
	ReasonCode   string                 `json:"reason_code,omitempty"`
	ReadChunkIDs []string               `json:"read_chunk_ids,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type CoverageState string

const (
	CoverageMissing    CoverageState = "missing"
	CoveragePartial    CoverageState = "partial"
	CoverageSufficient CoverageState = "sufficient"
	CoverageContested  CoverageState = "contested"
)

type CoverageItem struct {
	Dimension string        `json:"dimension"`
	State     CoverageState `json:"state"`
	Summary   string        `json:"summary,omitempty"`
	SourceIDs []string      `json:"source_ids,omitempty"`
	ChunkIDs  []string      `json:"chunk_ids,omitempty"`
}

type CoverageMatrix struct {
	Items     []CoverageItem `json:"items"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type EvidenceConflict struct {
	ID          string    `json:"id"`
	Topic       string    `json:"topic"`
	SourceIDs   []string  `json:"source_ids"`
	ChunkIDs    []string  `json:"chunk_ids,omitempty"`
	Disposition string    `json:"disposition,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type BlueprintSection struct {
	Key      string   `json:"key"`
	Content  string   `json:"content"`
	ClaimIDs []string `json:"claim_ids,omitempty"`
	ChunkIDs []string `json:"chunk_ids,omitempty"`
}

// BlueprintSections accepts either the contracted array shape or a single
// section object. Models occasionally collapse a one-item array into an object;
// normalizing that harmless shape difference keeps evidence validation in charge.
type BlueprintSections []BlueprintSection

func (s *BlueprintSections) UnmarshalJSON(data []byte) error {
	var list []BlueprintSection
	if err := json.Unmarshal(data, &list); err == nil {
		*s = list
		return nil
	}
	var single BlueprintSection
	if err := json.Unmarshal(data, &single); err == nil && (cleanText(single.Key) != "" || cleanText(single.Content) != "" || len(single.ClaimIDs) > 0 || len(single.ChunkIDs) > 0) {
		*s = BlueprintSections{single}
		return nil
	}
	var keyed map[string]BlueprintSection
	if err := json.Unmarshal(data, &keyed); err != nil {
		return fmt.Errorf("relationships must be an array or object: %w", err)
	}
	keys := make([]string, 0, len(keyed))
	for key := range keyed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	list = make([]BlueprintSection, 0, len(keys))
	for _, key := range keys {
		section := keyed[key]
		if cleanText(section.Key) == "" {
			section.Key = key
		}
		list = append(list, section)
	}
	*s = list
	return nil
}

type RoleBlueprint struct {
	ID                    string            `json:"id"`
	RunID                 string            `json:"run_id"`
	RoleID                string            `json:"role_id"`
	Version               int64             `json:"version"`
	TargetPeriod          string            `json:"target_period"`
	KnowledgeCutoff       string            `json:"knowledge_cutoff"`
	SelfConcept           BlueprintSection  `json:"self_concept"`
	ValuesAndMotives      BlueprintSection  `json:"values_and_motives"`
	Tensions              BlueprintSection  `json:"tensions"`
	Relationships         BlueprintSections `json:"relationships,omitempty"`
	ReasoningAndVoice     BlueprintSection  `json:"reasoning_and_voice"`
	UnknownResponsePolicy BlueprintSection  `json:"unknown_response_policy"`
	AllowedInferences     BlueprintSection  `json:"allowed_inferences"`
	ForbiddenAnachronisms BlueprintSection  `json:"forbidden_anachronisms"`
	ChangeSummary         string            `json:"change_summary,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`
}

type CritiqueSeverity string

const (
	CritiqueHard    CritiqueSeverity = "hard"
	CritiqueWarning CritiqueSeverity = "warning"
)

type CritiqueIssue struct {
	Code     string           `json:"code"`
	Severity CritiqueSeverity `json:"severity"`
	Message  string           `json:"message"`
	Section  string           `json:"section,omitempty"`
	ClaimIDs []string         `json:"claim_ids,omitempty"`
	ChunkIDs []string         `json:"chunk_ids,omitempty"`
	Resolved bool             `json:"resolved,omitempty"`
}

type RoleCritique struct {
	ID                      string          `json:"id"`
	RunID                   string          `json:"run_id"`
	BlueprintID             string          `json:"blueprint_id"`
	Issues                  []CritiqueIssue `json:"issues,omitempty"`
	Passed                  bool            `json:"passed"`
	CreatedAt               time.Time       `json:"created_at"`
	WarningAcceptanceReason string          `json:"warning_acceptance_reason,omitempty"`
	ApprovedAt              *time.Time      `json:"approved_at,omitempty"`
}

type RoleInitializationRun struct {
	ID                  string                    `json:"id"`
	RoleID              string                    `json:"role_id"`
	VariantOfRoleID     string                    `json:"variant_of_role_id,omitempty"`
	Status              InitializationStatus      `json:"status"`
	ResumeStatus        InitializationStatus      `json:"resume_status,omitempty"`
	CurrentStep         string                    `json:"current_step,omitempty"`
	Checkpoint          string                    `json:"checkpoint,omitempty"`
	Objective           string                    `json:"objective,omitempty"`
	Plan                *RoleResearchPlan         `json:"plan,omitempty"`
	Assessments         []SourceAssessment        `json:"assessments,omitempty"`
	Coverage            CoverageMatrix            `json:"coverage"`
	Conflicts           []EvidenceConflict        `json:"conflicts,omitempty"`
	BlueprintID         string                    `json:"blueprint_id,omitempty"`
	CritiqueID          string                    `json:"critique_id,omitempty"`
	RevisionRequest     string                    `json:"revision_request,omitempty"`
	RemoteBudget        int                       `json:"remote_budget"`
	RemoteUsed          int                       `json:"remote_used"`
	PrivateModelConsent bool                      `json:"private_model_consent,omitempty"`
	SearchProvider      string                    `json:"search_provider,omitempty"`
	ErrorSummary        string                    `json:"error_summary,omitempty"`
	Events              []RoleInitializationEvent `json:"events,omitempty"`
	Version             int64                     `json:"version"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
	CompletedAt         *time.Time                `json:"completed_at,omitempty"`
}

type RoleDocument struct {
	ID           string         `json:"id"`
	RoleID       string         `json:"role_id"`
	CorpusRoleID string         `json:"corpus_role_id"`
	SourceID     string         `json:"source_id,omitempty"`
	SourceURL    string         `json:"source_url,omitempty"`
	Title        string         `json:"title"`
	MimeType     string         `json:"mime_type,omitempty"`
	Audience     SourceAudience `json:"audience"`
	Tier         SourceTier     `json:"tier"`
	ContentHash  string         `json:"content_hash"`
	Path         string         `json:"path"`
	ChunkCount   int            `json:"chunk_count"`
	CreatedAt    time.Time      `json:"created_at"`
}

type RoleChunk struct {
	ID           string         `json:"id"`
	RoleID       string         `json:"role_id"`
	CorpusRoleID string         `json:"corpus_role_id"`
	DocumentID   string         `json:"document_id"`
	SourceID     string         `json:"source_id,omitempty"`
	Title        string         `json:"title"`
	Section      string         `json:"section,omitempty"`
	Page         int            `json:"page,omitempty"`
	Audience     SourceAudience `json:"audience"`
	Tier         SourceTier     `json:"tier"`
	ContentHash  string         `json:"content_hash"`
	Content      string         `json:"content,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type RoleChunkCard struct {
	ID         string         `json:"id"`
	DocumentID string         `json:"document_id"`
	SourceID   string         `json:"source_id,omitempty"`
	Title      string         `json:"title"`
	Section    string         `json:"section,omitempty"`
	Page       int            `json:"page,omitempty"`
	Audience   SourceAudience `json:"audience"`
	Tier       SourceTier     `json:"tier"`
	Excerpt    string         `json:"excerpt"`
}
