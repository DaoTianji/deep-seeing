package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"deep-seeing/internal/identity"
)

// HindsightClient uses only explicit REST operations. It does not auto-retain,
// reflect, retry billable requests, or send credentials to a redirect target.
type HindsightClient struct {
	base   string
	key    string
	client *http.Client
	mu     sync.Mutex
	usage  HindsightUsage
}

type HindsightUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
	Responses    int `json:"metered_responses"`
}

func (c *HindsightClient) Usage() HindsightUsage { c.mu.Lock(); defer c.mu.Unlock(); return c.usage }

type HindsightFact struct {
	DocumentID string            `json:"document_id"`
	Metadata   map[string]string `json:"metadata"`
}

func NewHindsightClient(base, key string) (*HindsightClient, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(base), "/"))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid Hindsight endpoint")
	}
	if u.Scheme == "http" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1" {
		return nil, fmt.Errorf("Hindsight requires HTTPS except on loopback")
	}
	return &HindsightClient{base: u.String(), key: key, client: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func HindsightBank(scope identity.TenantScope) string {
	b, _ := json.Marshal(scope)
	sum := sha256.Sum256(b)
	return "ds-episodes-v1-" + hex.EncodeToString(sum[:16])
}

func (c *HindsightClient) request(ctx context.Context, method, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("hindsight encode failed")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("hindsight request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	client := *c.client
	if strings.HasSuffix(path, "/memories") {
		client.Timeout = 180 * time.Second // one retain; never retry an uncertain write
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("hindsight transport unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("hindsight HTTP %d", resp.StatusCode)
	}
	if out == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 2<<20))
		return err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return fmt.Errorf("hindsight response exceeds bound")
	}
	if err = json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("hindsight malformed response")
	}
	return nil
}

func (c *HindsightClient) Recall(ctx context.Context, scope identity.TenantScope, query string) ([]HindsightFact, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	var out struct {
		Results *[]HindsightFact `json:"results"`
	}
	err := c.request(ctx, http.MethodPost, "/v1/default/banks/"+HindsightBank(scope)+"/memories/recall", map[string]any{
		"query": query, "types": []string{"world", "experience"}, "budget": "low", "max_tokens": 4096,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Results == nil {
		return nil, fmt.Errorf("hindsight missing results")
	}
	return *out.Results, nil
}

// RetainEpisode is intentionally not wired to chat writes. Callers must obtain
// explicit model-spend approval first. One bounded document; no automatic retry.
func (c *HindsightClient) RetainEpisode(ctx context.Context, scope identity.TenantScope, ep Episode) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if !OrdinaryEpisodeAllowed(ep, scope) {
		return fmt.Errorf("episode is outside ordinary recall scope")
	}
	if len(ep.Content) > 64*1024 {
		return fmt.Errorf("episode exceeds retain size bound")
	}
	var response struct {
		Success bool            `json:"success"`
		Bank    string          `json:"bank_id"`
		Count   int             `json:"items_count"`
		Async   bool            `json:"async"`
		Usage   *HindsightUsage `json:"usage"`
	}
	err := c.request(ctx, http.MethodPost, "/v1/default/banks/"+HindsightBank(scope)+"/memories", map[string]any{
		"async": false, "items": []any{map[string]any{
			"content": ep.Content, "document_id": ep.ID, "timestamp": ep.CreatedAt,
			"context":  "Source Episode; quoted speakers are not the memory bank agent. Timestamp is document record time, not necessarily event time. Experience mode: " + string(ep.ExperienceMode),
			"metadata": map[string]string{"episode_id": ep.ID, "revision": EpisodeRevision(ep)},
		}},
	}, &response)
	if err != nil {
		return err
	}
	if !response.Success || response.Bank != HindsightBank(scope) || response.Count != 1 || response.Async {
		return fmt.Errorf("hindsight retain not synchronously confirmed")
	}
	if response.Usage != nil {
		c.mu.Lock()
		c.usage.InputTokens += response.Usage.InputTokens
		c.usage.OutputTokens += response.Usage.OutputTokens
		c.usage.TotalTokens += response.Usage.TotalTokens
		c.usage.Responses++
		c.mu.Unlock()
	}
	return nil
}

// DeleteEpisode removes only a document in the caller's fixed bank. Source
// validation still rejects withdrawn memories immediately, before async cleanup.
func (c *HindsightClient) DeleteEpisode(ctx context.Context, scope identity.TenantScope, id string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("episode ID required")
	}
	err := c.request(ctx, http.MethodDelete, "/v1/default/banks/"+HindsightBank(scope)+"/documents/"+url.PathEscape(id), nil, nil)
	if err != nil && err.Error() == "hindsight HTTP 404" {
		return nil
	}
	return err
}
