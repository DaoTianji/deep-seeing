// Command check probes runtime connectivity (gateway / Redis / Neo4j / room).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/graph"
	"deep-seeing/internal/memory"
)

func main() {
	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var okN, failN, skipN int
	report := func(status, name, detail string) {
		switch status {
		case "OK":
			okN++
		case "FAIL":
			failN++
		default:
			skipN++
		}
		if detail == "" {
			fmt.Printf("%-4s %s\n", status, name)
			return
		}
		fmt.Printf("%-4s %s — %s\n", status, name, detail)
	}

	cfg := deepagent.ConfigFromEnv()
	if cfg.APIKey == "" || cfg.Model == "" {
		report("FAIL", "openai-gateway", "missing OPENAI_API_KEY or OPENAI_MODEL")
	} else {
		chat := &memory.ChatClient{
			APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 8,
			HTTPClient: &http.Client{Timeout: 45 * time.Second},
		}
		t0 := time.Now()
		out, err := chat.Complete(ctx, "Reply with exactly: pong", "ping")
		if err != nil {
			report("FAIL", "openai-gateway", err.Error())
		} else {
			report("OK", "openai-gateway", fmt.Sprintf("model=%s latency=%s reply=%q",
				cfg.Model, time.Since(t0).Round(time.Millisecond), trim(out, 40)))
		}
	}

	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		report("FAIL", "redis", "REDIS_ADDR empty")
	} else {
		db := 0
		fmt.Sscanf(strings.TrimSpace(os.Getenv("REDIS_DB")), "%d", &db)
		rdb := redis.NewClient(&redis.Options{
			Addr: addr, Password: os.Getenv("REDIS_PASSWORD"), DB: db,
			DialTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
		})
		t0 := time.Now()
		if err := rdb.Ping(ctx).Err(); err != nil {
			report("FAIL", "redis", err.Error())
		} else {
			report("OK", "redis", fmt.Sprintf("addr=%s latency=%s", addr, time.Since(t0).Round(time.Millisecond)))
		}
		_ = rdb.Close()
	}

	neoCfg, neoOK := graph.ConfigFromEnv()
	if !neoOK {
		report("SKIP", "neo4j", "disabled or missing NEO4J_*")
	} else {
		t0 := time.Now()
		store, err := graph.Open(ctx, neoCfg)
		if err != nil {
			report("FAIL", "neo4j", err.Error())
		} else {
			report("OK", "neo4j", fmt.Sprintf("uri=%s db=%s latency=%s",
				neoCfg.URI, neoCfg.Database, time.Since(t0).Round(time.Millisecond)))
			_ = store.Close(ctx)
		}
	}

	roomAddr := strings.TrimSpace(os.Getenv("ROOM_ADDR"))
	if roomAddr == "" {
		roomAddr = "127.0.0.1:3319"
	}
	roomURL := "http://" + roomAddr + "/api/runtime"
	t0 := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, roomURL, nil)
	if err != nil {
		report("FAIL", "room-api", err.Error())
	} else {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			report("FAIL", "room-api", fmt.Sprintf("%s not reachable: %v", roomURL, err))
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			if resp.StatusCode != 200 {
				report("FAIL", "room-api", fmt.Sprintf("status=%d body=%s", resp.StatusCode, trim(string(body), 120)))
			} else {
				var payload map[string]any
				_ = json.Unmarshal(body, &payload)
				rt, _ := payload["runtime"].(map[string]any)
				stores, _ := rt["stores"].(map[string]any)
				report("OK", "room-api", fmt.Sprintf("%s stores=%v latency=%s",
					roomURL, stores, time.Since(t0).Round(time.Millisecond)))
			}
		}
	}

	ddgURL := "https://api.duckduckgo.com/?q=connectivity+test&format=json&no_html=1"
	t0 = time.Now()
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, ddgURL, nil)
	if err != nil {
		report("FAIL", "duckduckgo-api", err.Error())
	} else {
		req.Header.Set("User-Agent", "deep-seeing-check/0.1")
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			report("FAIL", "duckduckgo-api", err.Error())
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			report("OK", "duckduckgo-api", fmt.Sprintf("status=%d bytes=%d latency=%s",
				resp.StatusCode, len(body), time.Since(t0).Round(time.Millisecond)))
		}
	}

	fmt.Printf("---\nsummary: OK=%d FAIL=%d SKIP=%d\n", okN, failN, skipN)
	if failN > 0 {
		os.Exit(1)
	}
}

func trim(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
