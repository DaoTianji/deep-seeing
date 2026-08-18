package world

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SearchHit is one search result.
type SearchHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// Searcher finds public web results (pluggable).
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]SearchHit, error)
}

// DuckDuckGoSearcher uses the Instant Answer JSON API (no key; limited coverage).
type DuckDuckGoSearcher struct {
	Client *SafeClient
	HTTP   *http.Client
}

// Search implements Searcher.
func (s *DuckDuckGoSearcher) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
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
	apiURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) + "&format=json&no_html=1&skip_disambig=1"
	if err := ValidateFetchURL(apiURL); err != nil {
		return nil, err
	}
	httpClient := s.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 8 * time.Second}
		if s.Client != nil && s.Client.HTTP != nil {
			// Keep redirect policy from SafeClient, but bound search latency tightly.
			httpClient = &http.Client{
				Timeout:       8 * time.Second,
				CheckRedirect: s.Client.HTTP.CheckRedirect,
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "deep-seeing-world/0.1")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var payload struct {
		Heading       string `json:"Heading"`
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
			Topics   []struct {
				Text     string `json:"Text"`
				FirstURL string `json:"FirstURL"`
			} `json:"Topics"`
		} `json:"RelatedTopics"`
		Results []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	var hits []SearchHit
	add := func(title, u, snip string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if err := ValidateFetchURL(u); err != nil {
			return
		}
		if title == "" {
			title = firstRunes(snip, 80)
		}
		hits = append(hits, SearchHit{Title: title, URL: u, Snippet: snip})
	}
	if payload.AbstractURL != "" {
		add(payload.Heading, payload.AbstractURL, payload.AbstractText)
	}
	for _, r := range payload.Results {
		add(firstRunes(r.Text, 80), r.FirstURL, r.Text)
		if len(hits) >= limit {
			break
		}
	}
	for _, t := range payload.RelatedTopics {
		if len(hits) >= limit {
			break
		}
		if t.FirstURL != "" {
			add(firstRunes(t.Text, 80), t.FirstURL, t.Text)
			continue
		}
		for _, sub := range t.Topics {
			if len(hits) >= limit {
				break
			}
			add(firstRunes(sub.Text, 80), sub.FirstURL, sub.Text)
		}
	}
	return hits, nil
}

// Gateway wires fetch + search + source persistence + budget.
type Gateway struct {
	Client  *SafeClient
	Sources *SourceStore
	Search  Searcher
	Budget  *FetchBudget
}

// NewGateway assembles defaults.
func NewGateway(sourceDir string) (*Gateway, error) {
	store, err := NewSourceStore(sourceDir)
	if err != nil {
		return nil, err
	}
	client := NewSafeClient()
	return &Gateway{
		Client: client, Sources: store, Budget: DefaultFetchBudget(),
		Search: &DuckDuckGoSearcher{Client: client},
	}, nil
}

// ReadWebpage fetches a URL, fences content, and persists a Source.
func (g *Gateway) ReadWebpage(ctx context.Context, rawURL string) (Source, error) {
	if g == nil || g.Client == nil || g.Sources == nil {
		return Source{}, fmt.Errorf("world gateway incomplete")
	}
	now := time.Now().UTC()
	if ok, why := g.Budget.Allow(now); !ok {
		return Source{}, fmt.Errorf("%s", why)
	}
	res, err := g.Client.Get(ctx, rawURL)
	if err != nil {
		return Source{}, err
	}
	g.Budget.Consume(now)
	text := res.Text
	if len([]rune(text)) > 12000 {
		text = string([]rune(text)[:12000]) + "…"
	}
	fenced := WrapUntrustedContent(res.FinalURL, text)
	title := firstRunes(text, 80)
	return g.Sources.Save(Source{
		URL: res.URL, FinalURL: res.FinalURL, Title: title,
		ContentType: res.ContentType, Body: fenced, FetchedAt: res.FetchedAt,
		Excerpt: firstRunes(text, 240),
	})
}

// SearchWeb runs a search and optionally persists a provenance note (not full pages).
func (g *Gateway) SearchWeb(ctx context.Context, query string, limit int) ([]SearchHit, Source, error) {
	if g == nil || g.Search == nil {
		return nil, Source{}, fmt.Errorf("searcher unavailable")
	}
	now := time.Now().UTC()
	if ok, why := g.Budget.Allow(now); !ok {
		return nil, Source{}, fmt.Errorf("%s", why)
	}
	hits, err := g.Search.Search(ctx, query, limit)
	if err != nil {
		return nil, Source{}, err
	}
	g.Budget.Consume(now)
	var b strings.Builder
	b.WriteString("search query: ")
	b.WriteString(query)
	b.WriteString("\nresults:\n")
	for i, h := range hits {
		fmt.Fprintf(&b, "%d. %s\n   %s\n   %s\n", i+1, h.Title, h.URL, h.Snippet)
	}
	fenced := WrapUntrustedContent("search:"+query, b.String())
	src, err := g.Sources.Save(Source{
		URL: "search://" + url.QueryEscape(query), Title: "search: " + query,
		Query: query, Body: fenced, FetchedAt: now, Excerpt: firstRunes(b.String(), 240),
	})
	return hits, src, err
}
