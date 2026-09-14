package story

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BudgetedCompleter bounds actual remote attempts, not user turns. Single process
// per data directory; reservations persist before calls, including failed calls.
type BudgetedCompleter struct {
	Next       Completer
	Root       string
	DailyLimit int
	mu         sync.Mutex
}

func (b *BudgetedCompleter) Complete(ctx context.Context, system, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(system)+len(input) > 2<<20 {
		return "", errors.New("本次资料超过演示版单次处理上限，请缩小范围")
	}
	if err := b.reserve(); err != nil {
		return "", err
	}
	return b.Next.Complete(ctx, system, input)
}
func (b *BudgetedCompleter) reserve() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	path := filepath.Join(b.Root, "limits", "model-calls.json")
	var state struct {
		Day  string `json:"day"`
		Used int    `json:"used"`
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(raw, &state) != nil || state.Used < 0 || state.Day == "" {
			return errors.New("演示用量记录异常，暂时停止生成；阅读不受影响")
		}
	} else if !os.IsNotExist(err) {
		return errors.New("暂时无法读取演示用量，阅读仍可使用")
	}
	day := time.Now().UTC().Format("2006-01-02")
	if state.Day != day {
		state.Day = day
		state.Used = 0
	}
	if b.DailyLimit <= 0 || state.Used >= b.DailyLimit {
		return errors.New("今日演示生成额度已用完，可继续阅读或查看预先生成的故事示例")
	}
	state.Used++
	if atomicJSON(path, state) != nil {
		return errors.New("暂时无法保存演示用量，未发起模型请求")
	}
	return nil
}

// PublicDemoGuard is opt-in for the isolated competition service. It never
// trusts forwarded IP headers. Global bounds also apply when visitors rename.
func PublicDemoGuard(next http.Handler) http.Handler {
	slots := make(chan struct{}, 3)
	var mu sync.Mutex
	var minute time.Time
	var writes int
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/story/authors") {
			jsonResponse(w, 404, map[string]string{"error": "作者工作台不在公开比赛版范围内"})
			return
		}
		// The public endpoint is HTTPS behind a loopback reverse proxy. Do not rely
		// on arbitrary X-Forwarded-* headers to decide cookie security.
		w = &secureCookieWriter{ResponseWriter: w}
		if r.Method == http.MethodPost {
			mu.Lock()
			now := time.Now().Truncate(time.Minute)
			if !now.Equal(minute) {
				minute = now
				writes = 0
			}
			writes++
			allowed := writes <= 120
			mu.Unlock()
			if !allowed {
				w.Header().Set("Retry-After", "60")
				jsonResponse(w, 429, map[string]string{"error": "当前操作较多，请一分钟后再试；已保存记录不会丢失"})
				return
			}
			expensive := strings.HasSuffix(r.URL.Path, "/turn") || strings.HasSuffix(r.URL.Path, "/chat") || strings.HasSuffix(r.URL.Path, "/translation") || r.URL.Path == "/api/story/reading"
			if expensive {
				select {
				case slots <- struct{}{}:
					defer func() { <-slots }()
				default:
					w.Header().Set("Retry-After", "15")
					jsonResponse(w, 429, map[string]string{"error": "演示服务正在处理其他请求，请稍后重试。可先阅读原文或查看故事示例"})
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

type secureCookieWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *secureCookieWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *secureCookieWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	values := w.Header().Values("Set-Cookie")
	w.Header().Del("Set-Cookie")
	for _, v := range values {
		if !strings.Contains(strings.ToLower(v), "; secure") {
			v += "; Secure"
		}
		w.Header().Add("Set-Cookie", v)
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *secureCookieWriter) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.ResponseWriter.Write(p)
}
