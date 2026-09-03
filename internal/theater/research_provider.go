package theater

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
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

// BingRoleSearch uses Bing's RSS result representation. It is intended for
// personal, non-commercial role research when no authenticated search API is
// configured; result cards are still only candidates and must be read before
// they can become evidence.
type BingRoleSearch struct {
	HTTP *http.Client
}

func (p *BingRoleSearch) Name() string { return "bing" }

func (p *BingRoleSearch) Search(ctx context.Context, query string, limit int) ([]world.SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	endpoint := "https://cn.bing.com/search?q=" + url.QueryEscape(query) + "&format=rss"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
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
		return nil, fmt.Errorf("bing search status %d", res.StatusCode)
	}
	var payload struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	hits := make([]world.SearchHit, 0, min(limit, len(payload.Channel.Items)))
	for _, item := range payload.Channel.Items {
		link := cleanText(item.Link)
		if link == "" || world.ValidateFetchURL(link) != nil {
			continue
		}
		snippet := html.UnescapeString(cleanText(item.Description))
		hits = append(hits, world.SearchHit{Title: cleanText(item.Title), URL: link, Snippet: snippet})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
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

// RoleSearchProviderFromEnv selects Brave when configured, Bing RSS as the
// practical no-key default, and keeps DuckDuckGo Instant Answer as an explicit
// limited compatibility option.
func RoleSearchProviderFromEnv(gateway *world.Gateway) (RoleSearchProvider, bool) {
	requested := strings.ToLower(strings.TrimSpace(os.Getenv("ROLE_SEARCH_PROVIDER")))
	key := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY"))
	if (requested == "" || requested == "brave") && key != "" {
		return &BraveRoleSearch{APIKey: key}, false
	}
	if requested == "bing" || requested == "" || requested == "brave" {
		return &BingRoleSearch{}, true
	}
	var searcher world.Searcher
	if gateway != nil {
		searcher = gateway.Search
	}
	return &DuckDuckGoRoleSearch{Searcher: searcher}, true
}
