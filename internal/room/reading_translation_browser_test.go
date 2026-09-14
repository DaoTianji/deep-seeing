package room

// Opt-in local UI fixture: no model gateway, no real reader data. Run with
// READING_TRANSLATION_BROWSER_ADDR=127.0.0.1:3324 go test ./internal/room
// -run TestReadingTranslationBrowserFixture -v, then stop with Ctrl-C.
import (
	"context"
	"deep-seeing/internal/story"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type translationBrowserModel struct{ calls atomic.Int64 }

func (m *translationBrowserModel) Complete(ctx context.Context, _ string, input string) (string, error) {
	var in struct {
		Sentences []story.TranslationSentence `json:"sentences"`
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil || len(in.Sentences) < 1 || len(in.Sentences) > 2 {
		return "", fmt.Errorf("fixture only accepts its two sentences")
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}
	call := m.calls.Add(1)
	items := []map[string]string{}
	for _, s := range in.Sentences {
		var text string
		switch s.ID {
		case "p1-s1":
			text = "怀特太太打开书。"
			if call > 1 {
				text = "怀特太太翻开了那本书。"
			}
		case "p1-s2":
			text = "窗外的雨停了。"
			if call > 1 {
				text = "外面的雨已经停了。"
			}
		default:
			return "", fmt.Errorf("unexpected fixture sentence")
		}
		items = append(items, map[string]string{"id": s.ID, "translation": text})
	}
	raw, _ := json.Marshal(map[string]any{"items": items})
	return string(raw), nil
}
func TestReadingTranslationBrowserFixture(t *testing.T) {
	addr := os.Getenv("READING_TRANSLATION_BROWSER_ADDR")
	if addr == "" {
		t.Skip("manual browser fixture")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("loopback required")
	}
	e, err := story.New(t.TempDir(), &translationBrowserModel{}, "fixture-no-model-cost", "agent")
	if err != nil {
		t.Fatal(err)
	}
	e.Book.Text = "Mrs. White opened the book. The rain stopped outside."
	e.Book.Title = "逐句翻译验收（虚构）"
	e.Book.Version = "translation-fixture"
	e.Book.ReadingStart = ""
	if _, err = e.TranslateBatch(context.Background(), uuid.NewString(), "fluent", 0); err != nil {
		t.Fatal(err)
	}
	h, err := ReadingHandler(e)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	t.Cleanup(func() { server.Close() })
	t.Logf("fixture ready http://%s/reading?book=necklace", addr)
	done := make(chan error, 1)
	go func() { done <- server.Serve(ln) }()
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Minute):
	}
}
