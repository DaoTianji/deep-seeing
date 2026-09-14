package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestUsageObserverRequiresCompleteReceipt(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		count int
	}{
		{`null`, 0}, {`{}`, 0}, {`{"total_tokens":12}`, 0},
		{`{"prompt_tokens":1,"completion_tokens":2,"total_tokens":null}`, 0},
		{`{"prompt_tokens":-1,"completion_tokens":2,"total_tokens":1}`, 0},
		{`{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}`, 1},
		{`{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}`, 1},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			count := 0
			ctx := WithChatUsageObserver(context.Background(), func(ChatUsage) { count++ })
			observeChatUsage(ctx, json.RawMessage(tc.raw))
			if count != tc.count {
				t.Fatalf("got %d want %d", count, tc.count)
			}
		})
	}
}

func TestUsageObserversIsolateConcurrentCallsAndCountTruncation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10},"choices":[{"finish_reason":"length","message":{"content":"not committed","reasoning_content":"PRIVATE"}}]}`))
	}))
	defer s.Close()
	c := &ChatClient{APIKey: "test", Model: "test", BaseURL: s.URL}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var received []ChatUsage
			ctx := WithChatUsageObserver(context.Background(), func(u ChatUsage) { received = append(received, u) })
			if _, err := c.Complete(ctx, "private prompt", "fiction"); err == nil {
				t.Error("truncation accepted")
			}
			if len(received) != 1 || received[0].TotalTokens != 10 {
				t.Error("receipt mixed between contexts")
			}
		}()
	}
	wg.Wait()
	if c.Usage().TotalTokens != 80 {
		t.Fatal("legacy total changed")
	}
}
