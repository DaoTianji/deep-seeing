package compaction

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/transcript"
)

const maxSingleMessageRunes = 4000

// SummarizingCompactor summarizes older turns with an LLM, keeps MinTail raw messages,
// and falls back to trim on failure.
type SummarizingCompactor struct {
	MaxTokens    int
	MaxMessages  int
	MinTail      int
	TriggerRatio float64
	Complete     Completer
	Fallback     *ThresholdCompactor
}

// NewSummarizingCompactor builds a summarizer with trim fallback.
func NewSummarizingCompactor(cfg Config, complete Completer) *SummarizingCompactor {
	fb := NewThresholdCompactor(cfg.MaxTokens, cfg.MaxMessages, cfg.MinTail)
	fb.TriggerRatio = cfg.TriggerRatio
	return &SummarizingCompactor{
		MaxTokens:    cfg.MaxTokens,
		MaxMessages:  cfg.MaxMessages,
		MinTail:      cfg.MinTail,
		TriggerRatio: cfg.TriggerRatio,
		Complete:     complete,
		Fallback:     fb,
	}
}

func (c *SummarizingCompactor) effectiveMaxTokens() int {
	max := 24000
	if c != nil && c.MaxTokens > 0 {
		max = c.MaxTokens
	}
	ratio := 0.8
	if c != nil && c.TriggerRatio > 0 && c.TriggerRatio <= 1 {
		ratio = c.TriggerRatio
	}
	return applyRatio(max, ratio)
}

func (c *SummarizingCompactor) effectiveMaxMessages() int {
	max := 32
	if c != nil && c.MaxMessages > 0 {
		max = c.MaxMessages
	}
	ratio := 0.8
	if c != nil && c.TriggerRatio > 0 && c.TriggerRatio <= 1 {
		ratio = c.TriggerRatio
	}
	return applyRatio(max, ratio)
}

func (c *SummarizingCompactor) effectiveMinTail() int {
	if c == nil || c.MinTail <= 0 {
		return 8
	}
	return c.MinTail
}

func (c *SummarizingCompactor) MaybeCompact(ctx context.Context, scope identity.TenantScope, sessionID string, msgs []transcript.Message, estTokens int) ([]transcript.Message, Report, error) {
	if c == nil {
		return msgs, Report{}, nil
	}
	msgs = truncateLongMessages(msgs, maxSingleMessageRunes)

	maxMsg := c.effectiveMaxMessages()
	minTail := c.effectiveMinTail()
	maxTok := c.effectiveMaxTokens()
	tok := estTokens
	if tok <= 0 {
		tok = RoughTokenEstimator(msgs)
	}
	i := 0
	for i < len(msgs) && msgs[i].Role == transcript.RoleSystem {
		i++
	}
	prefix := append([]transcript.Message(nil), msgs[:i]...)
	rest := append([]transcript.Message(nil), msgs[i:]...)
	if len(rest) == 0 {
		return msgs, Report{}, nil
	}
	if len(rest) <= maxMsg && tok <= maxTok {
		return msgs, Report{}, nil
	}
	if len(rest) <= minTail {
		return msgs, Report{}, nil
	}

	headEnd := len(rest) - minTail
	if headEnd <= 0 {
		return msgs, Report{}, nil
	}
	head := rest[:headEnd]
	tail := rest[headEnd:]

	if c.Complete == nil {
		return c.fallback(ctx, scope, sessionID, msgs, estTokens)
	}
	summary, err := c.summarize(ctx, head)
	if err != nil || strings.TrimSpace(summary) == "" {
		log.Printf("stm compact summarize failed, fallback trim: %v", err)
		return c.fallback(ctx, scope, sessionID, msgs, estTokens)
	}
	sumMsg := transcript.Summary(summary)
	out := append(append([]transcript.Message{}, prefix...), sumMsg)
	out = append(out, tail...)

	newTok := RoughTokenEstimator(out)
	if newTok > c.MaxTokens && c.MaxTokens > 0 && len(out) <= minTail+1 {
		return nil, Report{}, fmt.Errorf("compaction thrash: still over token budget after summarize (%d > %d)", newTok, c.MaxTokens)
	}
	// If still over budget with room to trim, trim further (lossy) once.
	if (len(out)-len(prefix) > maxMsg || newTok > maxTok) && len(out)-len(prefix) > minTail+1 {
		trimmed, changed := trimOldest(out, maxMsg, minTail, 0, maxTok)
		if changed {
			out = trimmed
		}
	}
	finalTok := RoughTokenEstimator(out)
	hardMax := c.MaxTokens
	if hardMax <= 0 {
		hardMax = 24000
	}
	if finalTok > hardMax && len(out)-len(prefix) <= minTail+1 {
		return nil, Report{}, fmt.Errorf("compaction thrash: still over token budget (%d > %d)", finalTok, hardMax)
	}
	return out, Report{Applied: true, Reason: "summarized older turns", WriteBack: true}, nil
}

func (c *SummarizingCompactor) fallback(ctx context.Context, scope identity.TenantScope, sessionID string, msgs []transcript.Message, estTokens int) ([]transcript.Message, Report, error) {
	fb := c.Fallback
	if fb == nil {
		fb = NewThresholdCompactor(c.MaxTokens, c.MaxMessages, c.MinTail)
		fb.TriggerRatio = c.TriggerRatio
	}
	return fb.MaybeCompact(ctx, scope, sessionID, msgs, estTokens)
}

func (c *SummarizingCompactor) summarize(ctx context.Context, head []transcript.Message) (string, error) {
	var b strings.Builder
	for _, m := range head {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	system := `你是会话压缩助手。将下列较早的对话压缩成一段简洁中文摘要，保留：约定称呼、已做决定、未完成事项、关键事实。不要编造。只输出摘要正文，不要标题或列表标记以外的废话。`
	return c.Complete.Complete(ctx, system, b.String())
}

func truncateLongMessages(msgs []transcript.Message, maxRunes int) []transcript.Message {
	if maxRunes <= 0 {
		return msgs
	}
	out := make([]transcript.Message, len(msgs))
	copy(out, msgs)
	for i := range out {
		if utf8.RuneCountInString(out[i].Content) <= maxRunes {
			continue
		}
		runes := []rune(out[i].Content)
		out[i].Content = string(runes[:maxRunes]) + "…"
	}
	return out
}
