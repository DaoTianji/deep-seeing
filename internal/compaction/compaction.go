package compaction

import (
	"context"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/transcript"
)

// Report describes compaction outcome.
type Report struct {
	Applied bool
	Reason  string
	// WriteBack is true when the caller should Replace the compacted history into STM.
	WriteBack bool
}

// Compactor optionally shrinks transcript before model calls.
type Compactor interface {
	MaybeCompact(ctx context.Context, scope identity.TenantScope, sessionID string, msgs []transcript.Message, estTokens int) ([]transcript.Message, Report, error)
}

// Completer is a minimal chat completion hook (avoids importing memory into compaction).
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// NoopCompactor does nothing.
type NoopCompactor struct{}

func (NoopCompactor) MaybeCompact(_ context.Context, _ identity.TenantScope, _ string, msgs []transcript.Message, _ int) ([]transcript.Message, Report, error) {
	return msgs, Report{}, nil
}

// RoughTokenEstimator approximates tokens as rune_count/3 + per-message overhead.
func RoughTokenEstimator(msgs []transcript.Message) int {
	n := 0
	for _, m := range msgs {
		n += 4 + utf8.RuneCountInString(m.Content)/3
	}
	return n
}

// ThresholdCompactor drops oldest non-system messages until under MaxTokens / MaxMessages.
type ThresholdCompactor struct {
	MaxTokens    int
	MaxMessages  int
	MinTail      int
	TriggerRatio float64 // effective thresholds = max * ratio; default 0.8
}

func NewThresholdCompactor(maxTokens, maxMessages, minTail int) *ThresholdCompactor {
	return &ThresholdCompactor{MaxTokens: maxTokens, MaxMessages: maxMessages, MinTail: minTail, TriggerRatio: 0.8}
}

func (c *ThresholdCompactor) effectiveMaxTokens() int {
	max := 24000
	if c != nil && c.MaxTokens > 0 {
		max = c.MaxTokens
	}
	return applyRatio(max, c.triggerRatio())
}

func (c *ThresholdCompactor) effectiveMaxMessages() int {
	max := 32
	if c != nil && c.MaxMessages > 0 {
		max = c.MaxMessages
	}
	return applyRatio(max, c.triggerRatio())
}

func (c *ThresholdCompactor) effectiveMinTail() int {
	if c == nil || c.MinTail <= 0 {
		return 8
	}
	return c.MinTail
}

func (c *ThresholdCompactor) triggerRatio() float64 {
	if c == nil || c.TriggerRatio <= 0 || c.TriggerRatio > 1 {
		return 0.8
	}
	return c.TriggerRatio
}

func applyRatio(max int, ratio float64) int {
	v := int(float64(max) * ratio)
	if v < 1 {
		return 1
	}
	return v
}

func (c *ThresholdCompactor) MaybeCompact(_ context.Context, _ identity.TenantScope, _ string, msgs []transcript.Message, estTokens int) ([]transcript.Message, Report, error) {
	if c == nil {
		return msgs, Report{}, nil
	}
	out, changed := trimOldest(msgs, c.effectiveMaxMessages(), c.effectiveMinTail(), estTokens, c.effectiveMaxTokens())
	if !changed {
		return msgs, Report{}, nil
	}
	return out, Report{Applied: true, Reason: "trimmed oldest non-system messages", WriteBack: true}, nil
}

func trimOldest(msgs []transcript.Message, maxMsg, minTail, estTokens, maxTok int) ([]transcript.Message, bool) {
	i := 0
	for i < len(msgs) && msgs[i].Role == transcript.RoleSystem {
		i++
	}
	prefix := append([]transcript.Message(nil), msgs[:i]...)
	rest := append([]transcript.Message(nil), msgs[i:]...)
	if len(rest) == 0 {
		return msgs, false
	}
	tok := estTokens
	if tok <= 0 {
		tok = RoughTokenEstimator(msgs)
	}
	changed := false
	for (len(rest) > maxMsg || tok > maxTok) && len(rest) > minTail+1 {
		rest = rest[1:]
		changed = true
		tok = RoughTokenEstimator(append(prefix, rest...))
	}
	if !changed {
		return msgs, false
	}
	return append(prefix, rest...), true
}

// ConfigFromEnv reads COMPACT_* settings.
type Config struct {
	MaxTokens    int
	MaxMessages  int
	MinTail      int
	TriggerRatio float64
}

func ConfigFromEnv() Config {
	return Config{
		MaxTokens:    envInt("COMPACT_MAX_TOKENS", 24000),
		MaxMessages:  envInt("COMPACT_MAX_MESSAGES", 32),
		MinTail:      envInt("COMPACT_MIN_TAIL", 8),
		TriggerRatio: envFloat("COMPACT_TRIGGER_RATIO", 0.8),
	}
}

func envInt(key string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func envFloat(key string, def float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64)
	if err != nil || v <= 0 || v > 1 {
		return def
	}
	return v
}
