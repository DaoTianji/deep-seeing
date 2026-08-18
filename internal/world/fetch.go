package world

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMaxBytes = 1 << 20 // 1 MiB
	defaultTimeout  = 15 * time.Second
	maxRedirects    = 5
)

// FetchResult is a safely retrieved remote document.
type FetchResult struct {
	URL         string
	FinalURL    string
	StatusCode  int
	ContentType string
	Body        []byte
	Text        string // best-effort plain text
	FetchedAt   time.Time
}

// SafeClient fetches http(s) URLs under SSRF / size / redirect policy.
type SafeClient struct {
	HTTP     *http.Client
	MaxBytes int64
	Timeout  time.Duration
}

// NewSafeClient builds a client with CheckRedirect re-validating every hop.
func NewSafeClient() *SafeClient {
	c := &SafeClient{MaxBytes: defaultMaxBytes, Timeout: defaultTimeout}
	c.HTTP = &http.Client{
		Timeout: c.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if err := ValidateRedirectURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	return c
}

// Get fetches url after SSRF validation.
func (c *SafeClient) Get(ctx context.Context, rawURL string) (FetchResult, error) {
	if c == nil {
		c = NewSafeClient()
	}
	if err := ValidateFetchURL(rawURL); err != nil {
		return FetchResult{}, err
	}
	max := c.MaxBytes
	if max <= 0 {
		max = defaultMaxBytes
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return FetchResult{}, err
	}
	req.Header.Set("User-Agent", "deep-seeing-world/0.1 (+local; respectful fetch)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.5")

	client := c.HTTP
	if client == nil {
		client = NewSafeClient().HTTP
	}
	res, err := client.Do(req)
	if err != nil {
		return FetchResult{}, err
	}
	defer res.Body.Close()

	final := res.Request.URL.String()
	if err := ValidateFetchURL(final); err != nil {
		return FetchResult{}, fmt.Errorf("final url blocked: %w", err)
	}
	ct := res.Header.Get("Content-Type")
	if !allowedContentType(ct) {
		return FetchResult{}, fmt.Errorf("content-type %q not allowed", ct)
	}
	limited := io.LimitReader(res.Body, max+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return FetchResult{}, err
	}
	if int64(len(body)) > max {
		return FetchResult{}, fmt.Errorf("response exceeds %d bytes", max)
	}
	text := ExtractText(ct, body)
	return FetchResult{
		URL: rawURL, FinalURL: final, StatusCode: res.StatusCode,
		ContentType: ct, Body: body, Text: text, FetchedAt: time.Now().UTC(),
	}, nil
}

func allowedContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "" {
		return true
	}
	switch {
	case strings.HasPrefix(ct, "text/"):
		return true
	case strings.Contains(ct, "html"):
		return true
	case strings.Contains(ct, "xml"):
		return true
	case strings.Contains(ct, "json"):
		return true
	default:
		return false
	}
}

// ExtractText returns plain-ish text from body.
func ExtractText(contentType string, body []byte) string {
	ct := strings.ToLower(contentType)
	s := string(body)
	if strings.Contains(ct, "html") || strings.Contains(ct, "xml") || looksLikeHTML(s) {
		return htmlToText(s)
	}
	return strings.TrimSpace(s)
}

func looksLikeHTML(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "<html") || strings.Contains(lower, "<body") || strings.Contains(lower, "<!doctype")
}

func htmlToText(s string) string {
	// crude strip: remove script/style blocks then tags
	lower := strings.ToLower(s)
	for _, tag := range []string{"script", "style", "noscript"} {
		for {
			start := strings.Index(lower, "<"+tag)
			if start < 0 {
				break
			}
			endTag := "</" + tag + ">"
			end := strings.Index(lower[start:], endTag)
			if end < 0 {
				s = s[:start]
				lower = strings.ToLower(s)
				break
			}
			end = start + end + len(endTag)
			s = s[:start] + " " + s[end:]
			lower = strings.ToLower(s)
		}
	}
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "\u00a0", " ")
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}
