package memory

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"deep-seeing/internal/identity"
)

// LLMExtractor uses a cheap completion to propose durable memories from a turn.
type LLMExtractor struct {
	Chat       *ChatClient
	LTM        Provider
	MinTurnLen int
}

type extractItem struct {
	Category string `json:"category"`
	Key      string `json:"key"`
	Content  string `json:"content"`
}

func (e *LLMExtractor) AfterTurn(ctx context.Context, scope identity.TenantScope, sessionID, turnUser, turnAssistant string) error {
	if e == nil || e.LTM == nil || e.Chat == nil {
		return nil
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	text := strings.TrimSpace(turnUser + "\n" + turnAssistant)
	minLen := e.MinTurnLen
	if minLen <= 0 {
		minLen = 40
	}
	if utf8.RuneCountInString(text) < minLen {
		return nil
	}
	system := strings.TrimSpace(`
你是记忆策展助手。根据对话片段，仅提取**值得跨轮次保留**的稳定事实（用户偏好、项目规则、明确反馈）。
不要保存：临时任务步骤、一次性命令、密钥与隐私。
输出必须是 JSON 数组，元素形如：
[{"category":"user|feedback|project|reference|person","key":"snake_case_id","content":"一句具体事实"}]
最多 5 条；若无值得保存的内容，输出 []。
只输出 JSON，不要 markdown 围栏。`)
	raw, err := e.Chat.Complete(ctx, system, "对话片段：\n"+text)
	if err != nil {
		return err
	}
	raw = stripJSONFence(raw)
	var items []extractItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return err
	}
	for _, it := range items {
		cat := normalizeCategory(it.Category)
		key := strings.TrimSpace(it.Key)
		content := strings.TrimSpace(it.Content)
		if key == "" || content == "" {
			continue
		}
		if err := e.LTM.Write(ctx, scope, Write{Category: cat, Key: key, Content: content}); err != nil {
			return err
		}
	}
	_ = sessionID
	return nil
}

func normalizeCategory(raw string) Category {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(CategoryFeedback):
		return CategoryFeedback
	case string(CategoryProject):
		return CategoryProject
	case string(CategoryRef):
		return CategoryRef
	case string(CategoryPerson):
		return CategoryPerson
	default:
		return CategoryUser
	}
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
