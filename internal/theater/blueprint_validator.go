package theater

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ValidateBlueprintEvidence enforces provenance and audience rules independently
// of the model critic. It never accepts search snippets or generated material as evidence.
func ValidateBlueprintEvidence(ctx context.Context, store *Store, corpus *CorpusStore, run RoleInitializationRun, blueprint RoleBlueprint) []CritiqueIssue {
	var issues []CritiqueIssue
	if cleanText(blueprint.TargetPeriod) == "" {
		issues = append(issues, hardIssue("target_period_missing", "", "角色塑造方案缺少明确人生时期"))
	}
	if cleanText(blueprint.KnowledgeCutoff) == "" {
		issues = append(issues, hardIssue("knowledge_cutoff_missing", "", "角色塑造方案缺少知识截止线"))
	}
	accepted := map[string]SourceAssessment{}
	readChunks := map[string]SourceAssessment{}
	for _, assessment := range run.Assessments {
		if assessment.Status != AssessmentAccepted {
			continue
		}
		if source, _, err := store.GetSource(ctx, assessment.SourceID); err != nil || source.RoleID != run.RoleID {
			issues = append(issues, CritiqueIssue{Code: "source_missing", Severity: CritiqueHard, Message: "采用来源不存在或不属于本角色"})
		}
		accepted[assessment.SourceID] = assessment
		for _, chunkID := range assessment.ReadChunkIDs {
			readChunks[chunkID] = assessment
		}
	}
	claims, _ := store.ListClaims(ctx, run.RoleID)
	claimSet := map[string]bool{}
	for _, claim := range claims {
		claimSet[claim.ID] = true
		for _, sourceID := range claim.SourceIDs {
			if _, ok := accepted[sourceID]; !ok && claim.Kind != ClaimUnknown {
				issues = append(issues, CritiqueIssue{Code: "claim_source_not_accepted", Severity: CritiqueHard, Message: "正式 Claim 引用了未采用来源", ClaimIDs: []string{claim.ID}})
			}
		}
	}
	for _, section := range blueprintSections(blueprint) {
		for _, claimID := range section.ClaimIDs {
			if !claimSet[claimID] {
				issues = append(issues, CritiqueIssue{Code: "claim_missing", Severity: CritiqueHard, Message: "方案引用了不存在的 Claim", Section: section.Key, ClaimIDs: []string{claimID}})
			}
		}
		for _, chunkID := range section.ChunkIDs {
			assessment, read := readChunks[chunkID]
			if !read {
				issues = append(issues, CritiqueIssue{Code: "chunk_not_read", Severity: CritiqueHard, Message: "方案引用了未读取的语料片段", Section: section.Key, ChunkIDs: []string{chunkID}})
				continue
			}
			chunk, err := corpus.ReadChunk(ctx, chunkID)
			if err != nil {
				issues = append(issues, CritiqueIssue{Code: "chunk_missing", Severity: CritiqueHard, Message: "方案引用的语料片段不存在", Section: section.Key, ChunkIDs: []string{chunkID}})
				continue
			}
			if chunk.SourceID != assessment.SourceID {
				issues = append(issues, CritiqueIssue{Code: "chunk_source_mismatch", Severity: CritiqueHard, Message: "读取片段与声明采用的来源不一致", Section: section.Key, ChunkIDs: []string{chunkID}})
			}
			if chunk.RoleID != run.RoleID {
				issues = append(issues, CritiqueIssue{Code: "cross_role_evidence", Severity: CritiqueHard, Message: "方案引用了其他角色的语料", Section: section.Key, ChunkIDs: []string{chunkID}})
			}
			if chunk.Audience == SourceDirector && sectionActorVisible(section.Key) {
				issues = append(issues, CritiqueIssue{Code: "audience_leak", Severity: CritiqueHard, Message: "台前角色内容引用了仅导演可见材料", Section: section.Key, ChunkIDs: []string{chunkID}})
			}
			if assessment.Tier == SourceGenerated || chunk.Tier == SourceGenerated {
				issues = append(issues, CritiqueIssue{Code: "generated_evidence", Severity: CritiqueHard, Message: "生成内容不能证明角色事实", Section: section.Key, ChunkIDs: []string{chunkID}})
			}
		}
		if containsDirectQuote(section.Content) && len(section.ChunkIDs) == 0 {
			issues = append(issues, CritiqueIssue{Code: "quote_unlocated", Severity: CritiqueHard, Message: "直接引语没有关联精确语料位置", Section: section.Key})
		}
	}
	if len(accepted) == 0 {
		issues = append(issues, hardIssue("no_accepted_sources", "", "没有已读取并采用的来源"))
	}
	if definition, err := store.GetDefinition(ctx, run.RoleID); err == nil && definition.Kind == RoleCharacter && definition.SubjectClass != SubjectFictional {
		for _, check := range []struct {
			code, label string
			section     BlueprintSection
		}{
			{"readiness_self_concept", "角色如何理解自己", blueprint.SelfConcept},
			{"readiness_values", "稳定价值与动机", blueprint.ValuesAndMotives},
			{"readiness_voice", "思考、论证与语言习惯", blueprint.ReasoningAndVoice},
		} {
			if !blueprintSectionSubstantive(check.section) {
				issues = append(issues, CritiqueIssue{Code: check.code, Severity: CritiqueWarning, Section: check.section.Key, Message: check.label + "仍缺少正面、可追溯的塑造内容；角色可以保持真实，但沉浸与回应能力会明显受限"})
			}
		}
	}
	return dedupeCritiqueIssues(issues)
}

func blueprintSectionSubstantive(section BlueprintSection) bool {
	if cleanText(section.Content) == "" || len(section.ChunkIDs) == 0 {
		return false
	}
	lower := strings.ToLower(section.Content)
	for _, marker := range []string{"资料不足", "证据不足", "现有材料不足", "尚未", "无法", "不足以", "不能建立", "unknown", "insufficient", "cannot establish"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func hardIssue(code, section, message string) CritiqueIssue {
	return CritiqueIssue{Code: code, Severity: CritiqueHard, Section: section, Message: message}
}

func blueprintSections(value RoleBlueprint) []BlueprintSection {
	sections := []BlueprintSection{value.SelfConcept, value.ValuesAndMotives, value.Tensions, value.ReasoningAndVoice, value.UnknownResponsePolicy, value.AllowedInferences, value.ForbiddenAnachronisms}
	sections = append(sections, value.Relationships...)
	return sections
}

func sectionActorVisible(key string) bool {
	switch cleanText(key) {
	case "forbidden_anachronisms", "historical_limits", "director_notes":
		return false
	default:
		return true
	}
}

func containsDirectQuote(value string) bool {
	return strings.Contains(value, "“") || strings.Contains(value, "”") || strings.Count(value, `"`) >= 2
}

func dedupeCritiqueIssues(in []CritiqueIssue) []CritiqueIssue {
	seen := map[string]bool{}
	out := make([]CritiqueIssue, 0, len(in))
	for _, issue := range in {
		key := fmt.Sprintf("%s|%s|%v|%v", issue.Code, issue.Section, issue.ClaimIDs, issue.ChunkIDs)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, issue)
	}
	return out
}

func newCritique(run RoleInitializationRun, blueprint RoleBlueprint, issues []CritiqueIssue) RoleCritique {
	return RoleCritique{ID: "rcrit_" + compactUUID(), RunID: run.ID, BlueprintID: blueprint.ID, Issues: dedupeCritiqueIssues(issues), Passed: !hasUnresolvedHardIssue(issues), CreatedAt: time.Now().UTC()}
}
