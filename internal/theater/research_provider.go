package theater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"deep-seeing/internal/world"
)

// RoleSearchProvider is the replaceable public-source search boundary used by
// Character Architect. Search results are candidates, never evidence.
type RoleSearchProvider interface {
	Name() string
	Search(context.Context, string, int) ([]world.SearchHit, error)
}

type DuckDuckGoRoleSearch struct{ Searcher world.Searcher }

func (p *DuckDuckGoRoleSearch) Name() string { return "duckduckgo" }
func (p *DuckDuckGoRoleSearch) Search(ctx context.Context, query string, limit int) ([]world.SearchHit, error) {
	if p == nil || p.Searcher == nil {
		return nil, fmt.Errorf("duckduckgo search unavailable")
	}
	return p.Searcher.Search(ctx, query, limit)
}

type BraveRoleSearch struct {
	APIKey string
	HTTP   *http.Client
}

func (p *BraveRoleSearch) Name() string { return "brave" }
func (p *BraveRoleSearch) Search(ctx context.Context, query string, limit int) ([]world.SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, fmt.Errorf("brave search api key required")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	endpoint := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query) + "&count=" + fmt.Sprint(limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", strings.TrimSpace(p.APIKey))
	req.Header.Set("User-Agent", "deep-seeing-role-research/0.1")
	client := p.HTTP
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("brave search status %d", res.StatusCode)
	}
	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	hits := make([]world.SearchHit, 0, len(payload.Web.Results))
	for _, item := range payload.Web.Results {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		hits = append(hits, world.SearchHit{Title: cleanText(item.Title), URL: cleanText(item.URL), Snippet: cleanText(item.Description)})
	}
	return hits, nil
}

// RoleSearchProviderFromEnv selects Brave when configured and safely falls
// back to the limited DuckDuckGo adapter when no key is available.
func RoleSearchProviderFromEnv(gateway *world.Gateway) (RoleSearchProvider, bool) {
	requested := strings.ToLower(strings.TrimSpace(os.Getenv("ROLE_SEARCH_PROVIDER")))
	key := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY"))
	if (requested == "" || requested == "brave") && key != "" {
		return &BraveRoleSearch{APIKey: key}, false
	}
	var searcher world.Searcher
	if gateway != nil {
		searcher = gateway.Search
	}
	return &DuckDuckGoRoleSearch{Searcher: searcher}, true
}
