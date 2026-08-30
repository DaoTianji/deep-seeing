package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/transcript"
)

func (r *SessionReviewer) runReflection(ctx context.Context, scope identity.TenantScope, sessionID string, history []transcript.Message, mode ReflectionMode) (ReviewResult, error) {
	if r.Reflections == nil {
		return ReviewResult{Mode: string(mode), Skipped: true, Reason: "reflection store unavailable"}, nil
	}
	if err := scope.Validate(); err != nil {
		return ReviewResult{}, err
	}
	reviewID := "review_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	result := ReviewResult{ReviewID: reviewID, Mode: string(mode)}
	lastTurnID, turnIDs := reviewTurnIDs(history)
	if cp, err := r.Reflections.LoadCheckpoint(scope.PersonID(), sessionID); err == nil && cp.MessageCount >= len(history) && (lastTurnID == "" || cp.LastTurnID == lastTurnID) {
		result.Skipped, result.Reason, result.Notes = true, "already reviewed", "No change."
		return result, nil
	}
	dialog := formatDialog(history)
	minChars := r.MinChars
	if minChars <= 0 {
		minChars = 80
	}
	if utf8.RuneCountInString(dialog) < minChars {
		result.Skipped, result.Reason = true, "session too short"
		return result, nil
	}

	bondText := "(no bond)"
	if r.Graph != nil {
		if bond, err := r.Graph.GetBond(ctx, scope, scope.PersonID()); err == nil {
			if text := strings.TrimSpace(bond.FormatRecall()); text != "" {
				bondText = text
			}
		}
		if err := r.Graph.TouchLastSeen(ctx, scope, scope.PersonID()); err != nil {
			log.Printf("session reflection touch last_seen: %v", err)
		}
	}

	system := strings.TrimSpace(`
你在为 Deep-Seeing 提供一次 Session Review。目标不是总结对话，也不是立刻修改长期认识，而是判断是否留下以后值得重新审视的问题。

默认 worth_review=false；普通任务、闲聊、一次性的情绪或已经明确处理完的事项不需要 Seed。
只有出现用户明确纠正、跨情境可能重复的关系模式、自我模式、无法解释的矛盾或值得长期验证的原则候选时才创建 ReflectionSeed。

硬规则：
1. 事实、观察、推断必须区分；Session Review 自己的判断通常是 model_inference。
2. 临时状态只写 state_observation，不泛化为人格。
3. roleplay/story/simulation 必须保留 experience_mode，不能伪装为 real_interaction。
4. 不产生 Proposal，不直接修改 Bond/Self/Soul。
5. scope 仅可为 bond|self_pattern|tension|principle_candidate。
6. source_type 仅可为 direct_expression|behavior_observation|model_inference。
7. 只输出 JSON，不要 markdown。

{
  "worth_review": false,
  "deviation": false,
  "hypothesis": "none|H1|H2",
  "state_observation": "",
  "reflection_seeds": [{"scope":"bond","statement":"一个短问题或待验证命题","source_type":"model_inference","experience_modes":["real_interaction"]}],
  "notes": "No change. 或简短公开说明"
}`)
	selfContext := loadReflectionContext(ctx, r.Context, scope)
	user := fmt.Sprintf("person=%s session=%s\n\n## 当前 Bond\n%s\n\n## 当前 Self Pattern / Tension 薄快照（不是历史证据）\n%s\n\n## 尚未持久化的本会话对话\n%s", scope.PersonID(), sessionID, bondText, mustJSON(selfContext), dialog)
	raw, err := r.Chat.Complete(ctx, system, user)
	if err != nil {
		return result, err
	}
	var out reviewModelOut
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &out); err != nil {
		return result, fmt.Errorf("parse reflection review json: %w\nraw=%s", err, truncate(raw, 400))
	}
	result.Hypothesis, result.Notes = string(normalizeHypothesis(out.Hypothesis)), strings.TrimSpace(out.Notes)
	if !out.WorthReview {
		result.Skipped, result.Reason = true, "model: nothing worth reviewing"
		if result.Notes == "" {
			result.Notes = "No change."
		}
		_ = r.saveReviewCheckpoint(scope, sessionID, lastTurnID, len(history))
		return result, nil
	}

	if obs := strings.TrimSpace(out.StateObservation); obs != "" {
		ep, err := r.Episodes.WriteEpisode(ctx, scope, EpisodeWrite{
			Kind: EpisodeStateObservation, ExperienceMode: ExperienceSelfReflection, Content: obs,
			Why: "session_review", PersonIDs: []string{scope.PersonID()}, SessionID: sessionID,
			Metadata: map[string]string{"source": "session_review", "epistemic": "derived_inference", "review_id": reviewID},
		})
		if err != nil {
			return result, err
		}
		result.StateObservationID = ep.ID
		if r.Graph != nil {
			_ = r.Graph.UpsertEpisodePointer(ctx, scope, graph.EpisodePointer{
				ID: ep.ID, Kind: string(ep.Kind), Summary: graph.SummaryFromContent(ep.Content, 160),
				DocURI: "by_id/" + ep.ID + ".md", SessionID: ep.SessionID, CreatedAt: ep.CreatedAt,
				PersonIDs: ep.PersonIDs, ExperienceMode: string(ep.ExperienceMode), Status: string(ep.Status),
			})
		}
	}
	for _, item := range out.Seeds {
		if strings.TrimSpace(item.Statement) == "" {
			continue
		}
		modes := make([]ExperienceMode, 0, len(item.ExperienceModes))
		for _, rawMode := range item.ExperienceModes {
			modes = append(modes, NormalizeExperienceMode(rawMode))
		}
		seed, err := r.Reflections.Create(ctx, scope, ReflectionSeedWrite{
			SessionID: sessionID, SourceTurnIDs: turnIDs, Scope: ReflectionScope(item.Scope),
			Statement: item.Statement, SourceType: ReflectionSource(item.SourceType), ExperienceModes: modes,
		})
		if err != nil {
			log.Printf("session review reflection seed: %v", err)
			continue
		}
		result.ReflectionSeedIDs = append(result.ReflectionSeedIDs, seed.ID)
	}
	if len(result.ReflectionSeedIDs) == 0 && result.StateObservationID == "" {
		result.Skipped, result.Reason = true, "model: no valid reflection seed"
		if result.Notes == "" {
			result.Notes = "No change."
		}
	}
	if err := r.saveReviewCheckpoint(scope, sessionID, lastTurnID, len(history)); err != nil {
		return result, err
	}
	return result, nil
}

func (r *SessionReviewer) saveReviewCheckpoint(scope identity.TenantScope, sessionID, lastTurnID string, count int) error {
	return r.Reflections.SaveCheckpoint(ReviewCheckpoint{
		PersonID: scope.PersonID(), SessionID: sessionID, LastTurnID: lastTurnID, MessageCount: count,
	})
}

func reviewTurnIDs(history []transcript.Message) (string, []string) {
	var ids []string
	for _, msg := range history {
		if id := strings.TrimSpace(msg.TurnID); id != "" {
			ids = append(ids, id)
		}
	}
	ids = uniqueStrings(ids)
	if len(ids) == 0 {
		return "", nil
	}
	return ids[len(ids)-1], ids
}
