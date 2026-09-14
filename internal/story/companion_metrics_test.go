package story

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"deep-seeing/internal/memory"
	"github.com/google/uuid"
)

func TestCompanionMetricsPersistButReplayAndTranslationCacheCostNothing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"usage":   map[string]int{"prompt_tokens": 12, "completion_tokens": 4, "total_tokens": 16},
			"choices": []any{map[string]any{"message": map[string]string{"content": `{"reply":"一段译文","evidence":[]}`}}},
		})
	}))
	defer server.Close()
	e, _, owner := companionFixture(t)
	e.Chat = &memory.ChatClient{BaseURL: server.URL, APIKey: "test", Model: "test"}
	in := companionRequest()
	in.Action = "translate"
	var attempt CompanionMetrics
	emit := func(kind, data string) {
		if kind == "metrics" {
			attempt = CompanionMetrics{}
			if err := json.Unmarshal([]byte(data), &attempt); err != nil {
				t.Fatal(err)
			}
		}
	}
	s, err := e.CompanionTurn(context.Background(), owner, in, emit)
	if err != nil {
		t.Fatal(err)
	}
	m := s.Turns[0].Metrics
	if m == nil || m.ModelCalls != 1 || m.ReportedCalls != 1 || m.Usage == nil || m.Usage.TotalTokens != 16 {
		t.Fatalf("missing exact receipt: %+v", m)
	}
	for _, replay := range []bool{true, false} {
		if !replay {
			in.RequestID = uuid.NewString()
			in.Revision = s.Revision
		}
		restored, err := e.CompanionTurn(context.Background(), owner, in, emit)
		if err != nil || attempt.ModelCalls != 0 || attempt.Usage != nil || len(restored.Turns) != 1 {
			t.Fatalf("replay/cache charged again: %+v %v", attempt, err)
		}
		if restored.Turns[0].Metrics.Usage.TotalTokens != 16 {
			t.Fatal("historical generation metrics overwritten")
		}
	}
	restart, _ := New(e.Root, nil, "test", "observe")
	restart.Book = e.Book
	s, err = restart.CompanionState(owner)
	if err != nil || s.Turns[0].Metrics.Usage.TotalTokens != 16 {
		t.Fatal("metrics not durable")
	}
}

func TestCompanionMetricsUnavailableAndExcludedFromModelHistory(t *testing.T) {
	e, c, owner := companionFixture(t, `{"reply":"一种解读","evidence":[1]}`, `{"ok":true}`, `{"reply":"继续理解","evidence":[1]}`, `{"ok":true}`)
	in := companionRequest()
	s, err := e.CompanionTurn(context.Background(), owner, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m := s.Turns[0].Metrics; m.ModelCalls != 2 || m.ReportedCalls != 0 || m.Usage != nil {
		t.Fatalf("invented token usage: %+v", m)
	}
	in.RequestID = uuid.NewString()
	in.Revision = s.Revision
	if _, err = e.CompanionTurn(context.Background(), owner, in, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.calls[2], "model_calls") {
		t.Fatal("operational counters leaked into literary context")
	}
}

func TestCompanionFailedTurnStreamsMetricsWithoutSavingAnswer(t *testing.T) {
	e, _, owner := companionFixture(t, `not valid JSON`)
	mux := http.NewServeMux()
	e.registerCatalog(mux)
	raw, _ := json.Marshal(companionRequest())
	r := httptest.NewRequest("POST", "/api/story/companion/chat", strings.NewReader(string(raw)))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: "reading_visitor", Value: owner})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var kinds []string
	for _, line := range strings.Split(strings.TrimSpace(w.Body.String()), "\n") {
		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, event.Type)
		if event.Type == "metrics" {
			var m CompanionMetrics
			_ = json.Unmarshal(event.Data, &m)
			if m.ModelCalls != 1 || m.Usage != nil {
				t.Fatal("bad failure metrics")
			}
		}
	}
	if !strings.Contains(fmt.Sprint(kinds), "metrics error") {
		t.Fatalf("missing metrics before failure: %v", kinds)
	}
	s, _ := e.CompanionState(owner)
	if len(s.Turns) != 0 || s.Revision != 0 {
		t.Fatal("failure mutated state")
	}
}
