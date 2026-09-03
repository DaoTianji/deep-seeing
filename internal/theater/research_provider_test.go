package theater

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roleSearchRoundTripper func(*http.Request) (*http.Response, error)

func (f roleSearchRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBraveRoleSearchUsesAuthenticatedFixedEndpoint(t *testing.T) {
	provider := &BraveRoleSearch{APIKey: "secret", HTTP: &http.Client{Transport: roleSearchRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.search.brave.com" || req.Header.Get("X-Subscription-Token") != "secret" {
			t.Fatalf("unsafe request: %s token=%q", req.URL, req.Header.Get("X-Subscription-Token"))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"web":{"results":[{"title":"传记","url":"https://example.test/bio","description":"简介"}]}}`)), Header: make(http.Header)}, nil
	})}}
	hits, err := provider.Search(context.Background(), "阿德勒 生平", 3)
	if err != nil || len(hits) != 1 || hits[0].Title != "传记" {
		t.Fatalf("hits=%#v err=%v", hits, err)
	}
}

func TestBingRoleSearchParsesRSSResults(t *testing.T) {
	provider := &BingRoleSearch{HTTP: &http.Client{Transport: roleSearchRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "cn.bing.com" || req.URL.Query().Get("format") != "rss" {
			t.Fatalf("unexpected request: %s", req.URL)
		}
		body := `<?xml version="1.0"?><rss><channel><item><title>教育部考试大纲</title><link>https://93.184.216.34/syllabus</link><description>312 &amp; 心理学</description></item></channel></rss>`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	hits, err := provider.Search(context.Background(), "312 心理学", 3)
	if err != nil || len(hits) != 1 || hits[0].Title != "教育部考试大纲" || hits[0].Snippet != "312 & 心理学" {
		t.Fatalf("hits=%#v err=%v", hits, err)
	}
}
