package memory

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type hindsightDeadlineTransport func(*http.Request) (*http.Response, error)

func (f hindsightDeadlineTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Inspect the actual outgoing deadline without waiting or calling a paid service.
func TestHindsightRequestTimeouts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		parent time.Duration
		want   time.Duration
	}{
		{"recall", "/v1/default/banks/test/memories/recall", 0, 60 * time.Second},
		{"retain unchanged", "/v1/default/banks/test/memories", 0, 180 * time.Second},
		{"caller deadline respected", "/v1/default/banks/test/memories/recall", 5 * time.Second, 5 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewHindsightClient("http://127.0.0.1:1", "")
			if err != nil {
				t.Fatal(err)
			}
			called := false
			client.client.Transport = hindsightDeadlineTransport(func(req *http.Request) (*http.Response, error) {
				called = true
				deadline, ok := req.Context().Deadline()
				if !ok {
					t.Fatal("request has no deadline")
				}
				if left := time.Until(deadline); left > tc.want || left < tc.want-time.Second {
					t.Fatalf("request timeout = %v, want approximately %v", left, tc.want)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			})
			ctx := context.Background()
			if tc.parent > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.parent)
				defer cancel()
			}
			if err := client.request(ctx, http.MethodPost, tc.path, nil, nil); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("transport was not called")
			}
			if client.client.Timeout != 60*time.Second {
				t.Fatal("per-request timeout mutated shared client")
			}
		})
	}
}
