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
	claimByID := map[string]RoleClaim{}
	for _, claim := range claims {
		claimSet[claim.ID] = true
		claimByID[claim.ID] = claim
		for _, sourceID := range claim.SourceIDs {
			if _, ok := accepted[sourceID]; !ok && claim.Kind != ClaimUnknown {
				issues = append(issues, CritiqueIssue{Code: "claim_source_not_accepted", Severity: CritiqueHard, Message: "正式 Claim 引用了未采用来源", ClaimIDs: []string{claim.ID}})
			}
		}
	}
	for _, section := range blueprintSections(blueprint) {
		sectionClaims := make([]RoleClaim, 0, len(section.ClaimIDs))
		claimChunks := map[string]bool{}
		for _, claimID := range section.ClaimIDs {
			claim, ok := claimByID[claimID]
			if !ok {
				issues = append(issues, CritiqueIssue{Code: "claim_missing", Severity: CritiqueHard, Message: "方案引用了不存在的 Claim", Section: section.Key, ClaimIDs: []string{claimID}})
				continue
			}
			sectionClaims = append(sectionClaims, claim)
			claimSources := map[string]bool{}
			quoteLocated := cleanText(claim.Quote) == ""
			selfConceptTierValid := claim.Kind != ClaimSelfConcept || len(claim.ChunkIDs) > 0
			for _, chunkID := range claim.ChunkIDs {
				claimChunks[chunkID] = true
				assessment, read := readChunks[chunkID]
				chunk, chunkErr := corpus.ReadChunk(ctx, chunkID)
				if !read || chunkErr != nil || chunk.RoleID != run.RoleID || chunk.Audience != SourceActor || chunk.SourceID != assessment.SourceID {
					issues = append(issues, CritiqueIssue{Code: "claim_chunk_invalid", Severity: CritiqueHard, Message: "Claim 没有关联本角色已读取的台前证据片段", Section: section.Key, ClaimIDs: []string{claimID}, ChunkIDs: []string{chunkID}})
					continue
				}
				claimSources[chunk.SourceID] = true
				if claim.Kind == ClaimSelfConcept && chunk.Tier != SourcePrimary && chunk.Tier != SourceContemporary {
					selfConceptTierValid = false
				}
				if cleanText(claim.Quote) != "" && strings.Contains(cleanText(chunk.Content), cleanText(claim.Quote)) {
					quoteLocated = true
				}
			}
			if claim.Kind == ClaimSelfConcept && !selfConceptTierValid {
				issues = append(issues, CritiqueIssue{Code: "self_concept_source_invalid", Severity: CritiqueHard, Message: "后世传记或评价不能支持角色第一人称自我认知", Section: section.Key, ClaimIDs: []string{claimID}})
			}
			if claim.Scope == ClaimScopeStablePattern && (len(claim.ChunkIDs) < 2 || len(claimSources) < 2) {
				issues = append(issues, CritiqueIssue{Code: "stable_pattern_evidence_insufficient", Severity: CritiqueHard, Message: "稳定模式必须由至少两个独立来源的多个片段支持", Section: section.Key, ClaimIDs: []string{claimID}})
			}
			if !quoteLocated {
				issues = append(issues, CritiqueIssue{Code: "claim_quote_unlocated", Severity: CritiqueHard, Message: "Claim 的直接引语未在关联片段中逐字出现", Section: section.Key, ClaimIDs: []string{claimID}})
			}
		}
		if blueprintSectionSubstantive(section) && sectionRequiresClaims(section.Key) && len(sectionClaims) == 0 {
			issues = append(issues, CritiqueIssue{Code: "section_claims_missing", Severity: CritiqueHard, Message: "正面人物塑造必须先引用可核验 Claim", Section: section.Key})
		}
		issues = append(issues, validateSectionClaimKinds(section, sectionClaims)...)
		for _, chunkID := range section.ChunkIDs {
			if blueprintSectionSubstantive(section) && sectionRequiresClaims(section.Key) && !claimChunks[chunkID] {
				issues = append(issues, CritiqueIssue{Code: "section_claim_mismatch", Severity: CritiqueHard, Message: "方案片段没有通过本节 Claim 建立证据关系", Section: section.Key, ChunkIDs: []string{chunkID}})
			}
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
		if containsDirectQuote(section.Content) && !sectionHasLocatedQuote(sectionClaims) {
			issues = append(issues, CritiqueIssue{Code: "quote_unlocated", Severity: CritiqueHard, Message: "直接引语必须由带 Quote 与 ChunkIDs 的 Claim 精确定位", Section: section.Key})
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

func sectionRequiresClaims(key string) bool {
	switch cleanText(key) {
	case "self_concept", "values_and_motives", "tensions", "reasoning_and_voice":
		return true
	default:
		return strings.HasPrefix(cleanText(key), "relationship")
	}
}

func validateSectionClaimKinds(section BlueprintSection, claims []RoleClaim) []CritiqueIssue {
	if !blueprintSectionSubstantive(section) || len(claims) == 0 {
		return nil
	}
	var issues []CritiqueIssue
	key := cleanText(section.Key)
	for _, claim := range claims {
		switch {
		case key == "self_concept" && claim.Kind != ClaimSelfConcept:
			issues = append(issues, CritiqueIssue{Code: "self_concept_claim_invalid", Severity: CritiqueHard, Message: "角色自我认知只能引用同期第一人称 self_concept Claim", Section: key, ClaimIDs: []string{claim.ID}})
		case strings.HasPrefix(key, "relationship") && claim.Kind != ClaimRelationship:
			issues = append(issues, CritiqueIssue{Code: "relationship_claim_invalid", Severity: CritiqueHard, Message: "关系事实必须引用 relationship Claim", Section: key, ClaimIDs: []string{claim.ID}})
		case key == "reasoning_and_voice" && (claim.Kind != ClaimVoice || claim.Scope != ClaimScopeStablePattern):
			issues = append(issues, CritiqueIssue{Code: "voice_generalization_invalid", Severity: CritiqueHard, Message: "稳定语言或推理习惯必须引用跨独立来源支持的 stable_pattern voice Claim", Section: key, ClaimIDs: []string{claim.ID}})
		}
	}
	return issues
}

func sectionHasLocatedQuote(claims []RoleClaim) bool {
	for _, claim := range claims {
		if cleanText(claim.Quote) != "" && len(claim.ChunkIDs) > 0 {
			return true
		}
	}
	return false
}

func blueprintSectionSubstantive(section BlueprintSection) bool {
	if cleanText(section.Content) == "" || len(section.ChunkIDs) == 0 {
		return false
	}
	lower := strings.ToLower(section.Content)
	for _, marker := range []string{"保持未知", "未知", "资料不足", "证据不足", "现有材料不足", "尚未", "无法", "不足以", "不能建立", "unknown", "insufficient", "cannot establish"} {
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
