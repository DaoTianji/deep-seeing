package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/transcript"
)

// SessionReviewer runs end-of-session review (Phase 3).
// It may write state_observation Episodes and enqueue Bond proposals,
// but never directly patches high-threshold Bond fields.
type SessionReviewer struct {
	Chat      *ChatClient
	Episodes  *EpisodeStore
	Proposals *ProposalStore
	Graph     GraphToucher // optional
	MinChars  int          // skip tiny sessions
}

// GraphToucher updates last_seen and can read bond for review context.
type GraphToucher interface {
	GetBond(ctx context.Context, scope identity.TenantScope, personID string) (graph.Bond, error)
	TouchLastSeen(ctx context.Context, scope identity.TenantScope, personID string) error
	UpsertEpisodePointer(ctx context.Context, scope identity.TenantScope, ep graph.EpisodePointer) error
}

// ReviewResult is a human-readable summary of what the review did.
type ReviewResult struct {
	Skipped           bool     `json:"skipped"`
	Reason            string   `json:"reason,omitempty"`
	Hypothesis        string   `json:"hypothesis,omitempty"`
	StateObservationID string  `json:"state_observation_id,omitempty"`
	ProposalIDs       []string `json:"proposal_ids,omitempty"`
	Notes             string   `json:"notes,omitempty"`
}

type reviewModelOut struct {
	WorthReview      bool   `json:"worth_review"`
	Deviation        bool   `json:"deviation"`
	Hypothesis       string `json:"hypothesis"`
	StateObservation string `json:"state_observation"`
	Notes            string `json:"notes"`
	Proposals        []struct {
		Field     string `json:"field"`
		Text      string `json:"text"`
		Mode      string `json:"mode"`
		Rationale string `json:"rationale"`
	} `json:"proposals"`
}

// Run reviews one session transcript against current Bond.
func (r *SessionReviewer) Run(ctx context.Context, scope identity.TenantScope, sessionID string, history []transcript.Message) (ReviewResult, error) {
	if r == nil || r.Episodes == nil || r.Proposals == nil || r.Chat == nil {
		return ReviewResult{Skipped: true, Reason: "reviewer incomplete"}, nil
	}
	if err := scope.Validate(); err != nil {
		return ReviewResult{}, err
	}
	dialog := formatDialog(history)
	minChars := r.MinChars
	if minChars <= 0 {
		minChars = 80
	}
	if utf8.RuneCountInString(dialog) < minChars {
		return ReviewResult{Skipped: true, Reason: "session too short"}, nil
	}

	bondText := "(no bond)"
	if r.Graph != nil {
		if bond, err := r.Graph.GetBond(ctx, scope, scope.PersonID()); err == nil {
			if t := strings.TrimSpace(bond.FormatRecall()); t != "" {
				bondText = t
			}
		}
		if err := r.Graph.TouchLastSeen(ctx, scope, scope.PersonID()); err != nil {
			log.Printf("session review touch last_seen: %v", err)
		}
	}

	system := strings.TrimSpace(`
你在为 Deep-Seeing 提供一次「会话回看机会」（Session Review），不是强制总结用户画像。

默认可以什么都不改变（worth_review=false / No change）。
仅当本会话里确实出现值得留下的波动观察或常模修订线索时，才提出 0..N 条提案。

硬规则：
1. 不得一次性整体改写性格/边界；style/boundaries 提案只能是 append 短句。
2. 出现偏离常模时给出双假设：H1（我的认知可能有偏）或 H2（对方状态可能异常）。
3. H2 时优先写 state_observation，常模提案通常 0 条。
4. H1 时可提案中门槛字段；高门槛只能 append。
5. 无价值 → worth_review=false。
6. 只输出 JSON，不要 markdown 围栏。

JSON 形状：
{
  "worth_review": false,
  "deviation": false,
  "hypothesis": "none|H1|H2",
  "state_observation": "",
  "proposals": [{"field":"basics","text":"...","mode":"append|replace","rationale":"..."}],
  "notes": "一两句；允许写 No change"
}`)
	user := fmt.Sprintf("person=%s session=%s\n\n## 当前 Bond\n%s\n\n## 本会话对话\n%s",
		scope.PersonID(), sessionID, bondText, dialog)

	raw, err := r.Chat.Complete(ctx, system, user)
	if err != nil {
		return ReviewResult{}, err
	}
	raw = stripJSONFence(raw)
	var out reviewModelOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return ReviewResult{}, fmt.Errorf("parse review json: %w\nraw=%s", err, truncate(raw, 400))
	}
	if !out.WorthReview {
		return ReviewResult{Skipped: true, Reason: "model: nothing worth reviewing", Notes: out.Notes, Hypothesis: out.Hypothesis}, nil
	}

	result := ReviewResult{
		Hypothesis: string(normalizeHypothesis(out.Hypothesis)),
		Notes:      strings.TrimSpace(out.Notes),
	}
	hyp := normalizeHypothesis(out.Hypothesis)

	if obs := strings.TrimSpace(out.StateObservation); obs != "" {
		ep, err := r.Episodes.WriteEpisode(ctx, scope, EpisodeWrite{
			Kind:      EpisodeStateObservation,
			Content:   obs,
			Why:       "session_review",
			PersonIDs: []string{scope.PersonID()},
			SessionID: sessionID,
			Metadata: map[string]string{
				"source":     "session_review",
				"hypothesis": string(hyp),
			},
		})
		if err != nil {
			return result, err
		}
		result.StateObservationID = ep.ID
		if r.Graph != nil {
			docURI := "by_id/" + ep.ID + ".md"
			if err := r.Graph.UpsertEpisodePointer(ctx, scope, graph.EpisodePointer{
				ID: ep.ID, Kind: string(ep.Kind), Summary: graph.SummaryFromContent(ep.Content, 160),
				DocURI: docURI, SessionID: ep.SessionID, CreatedAt: ep.CreatedAt, PersonIDs: ep.PersonIDs,
				ExperienceMode: string(ep.ExperienceMode), Status: string(ep.Status),
			}); err != nil {
				log.Printf("session review graph episode: %v", err)
			}
		}
	}

	for _, pr := range out.Proposals {
		field := normalizeProposalField(pr.Field)
		text := strings.TrimSpace(pr.Text)
		if field == "" || text == "" {
			continue
		}
		// H2: allow at most cautious proposals — skip high fields entirely
		if hyp == HypothesisH2 && (field == "style" || field == "boundaries") {
			continue
		}
		p, err := r.Proposals.Enqueue(ctx, scope, ProposalWrite{
			PersonID:      scope.PersonID(),
			SessionID:     sessionID,
			Hypothesis:    hyp,
			Field:         field,
			SuggestedText: text,
			Mode:          pr.Mode,
			Rationale:     pr.Rationale,
			Source:        "session_review",
		})
		if err != nil {
			log.Printf("session review enqueue: %v", err)
			continue
		}
		result.ProposalIDs = append(result.ProposalIDs, p.ID)
	}
	return result, nil
}

func formatDialog(history []transcript.Message) string {
	var b strings.Builder
	for _, m := range history {
		role := string(m.Role)
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(content)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
