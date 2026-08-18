package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"deep-seeing/internal/identity"
)

// LLMSideQuery selects relevant episode index entries with a small model, then reads episodes.
// Falls back to recent episodes when the chat client is unavailable.
type LLMSideQuery struct {
	Store EpisodeIndex
	Chat  *ChatClient
}

func (s *LLMSideQuery) SelectForTurn(ctx context.Context, scope identity.TenantScope, query string, limit int) ([]Record, error) {
	if s == nil || s.Store == nil {
		return nil, nil
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	index, err := s.Store.IndexText(ctx, scope)
	if err != nil {
		eps, listErr := s.Store.ListRecentEpisodes(ctx, scope, limit)
		if listErr != nil {
			return nil, fmt.Errorf("index: %w; recent: %v", err, listErr)
		}
		return episodesToRecords(eps), nil
	}
	if strings.TrimSpace(index) == "" || !strings.Contains(index, "- `") {
		return nil, nil
	}

	ids, err := s.selectIDs(ctx, query, index, limit)
	if err != nil || len(ids) == 0 {
		eps, err := s.Store.ListRecentEpisodes(ctx, scope, limit)
		if err != nil {
			return nil, err
		}
		return episodesToRecords(eps), nil
	}
	eps, err := s.Store.ReadIDs(ctx, scope, ids)
	if err != nil {
		return nil, err
	}
	if len(eps) == 0 {
		eps, err = s.Store.ListRecentEpisodes(ctx, scope, limit)
		if err != nil {
			return nil, err
		}
	}
	if len(eps) > limit {
		eps = eps[:limit]
	}
	return episodesToRecords(eps), nil
}

func episodesToRecords(eps []Episode) []Record {
	out := make([]Record, 0, len(eps))
	for _, ep := range eps {
		out = append(out, EpisodeToRecord(ep))
	}
	return out
}

func (s *LLMSideQuery) selectIDs(ctx context.Context, query, index string, limit int) ([]string, error) {
	if s.Chat == nil {
		return nil, fmt.Errorf("no chat client")
	}
	system := strings.TrimSpace(`
你是记忆旁路选型器。根据用户当前问题，从 Episode 索引里选出最相关的条目 id。
只输出 JSON 字符串数组，例如 ["ep_abc123"]。
最多选 limit 个；都不相关则输出 []。
不要解释，不要 markdown 围栏。`)
	user := fmt.Sprintf("limit=%d\n\n用户问题：\n%s\n\nEpisode 索引：\n%s", limit, query, index)
	raw, err := s.Chat.Complete(ctx, system, user)
	if err != nil {
		return nil, err
	}
	raw = stripJSONFence(raw)
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}
