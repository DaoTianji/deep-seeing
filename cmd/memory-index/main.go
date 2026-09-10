// memory-index inspects eligible source files by default. -apply is required
// for bounded, billable indexing. Never run concurrently with automatic sync.
package main

import (
	"context"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	dir := flag.String("episodes", "", "existing scoped Episode directory")
	journal := flag.String("journal", "", "persistent attempt journal (required for apply)")
	user := flag.String("user", "mudnet", "scope user")
	agent := flag.String("agent", "deep-seeing", "scope agent")
	apply := flag.Bool("apply", false, "perform bounded remote indexing")
	limit := flag.Int("limit", 3, "maximum documents this invocation")
	flag.Parse()
	if *dir == "" {
		panic("-episodes required")
	}
	if _, err := os.Stat(*dir + "/by_id"); err != nil {
		panic("existing episode directory required")
	}
	scope := identity.TenantScope{UserID: *user, AgentID: *agent}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	store, err := memory.NewEpisodeStore(*dir)
	must(err)
	eps, err := store.Snapshot(ctx)
	must(err)
	count, bytes := 0, 0
	for _, ep := range eps {
		if memory.OrdinaryEpisodeAllowed(ep, scope) {
			count++
			bytes += len(ep.Content)
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"eligible": count, "content_bytes": bytes, "bank": memory.HindsightBank(scope), "apply": *apply})
	if !*apply {
		return
	}
	if *journal == "" {
		panic("-journal required")
	}
	client, err := memory.NewHindsightClient(os.Getenv("HINDSIGHT_URL"), os.Getenv("HINDSIGHT_API_KEY"))
	must(err)
	syncer := &memory.IndexSync{Store: store, Scope: scope, Client: client, Path: *journal}
	p, err := syncer.SyncPending(ctx, *limit, 256*1024)
	counts := map[string]int{}
	usage := memory.HindsightUsage{}
	for _, e := range p.Entries {
		counts[e.Status]++
		if e.Usage != nil {
			usage.InputTokens += e.Usage.InputTokens
			usage.OutputTokens += e.Usage.OutputTokens
			usage.TotalTokens += e.Usage.TotalTokens
			usage.Responses += e.Usage.Responses
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"states": counts, "attempts_today": p.Attempts, "bytes_today": p.Bytes, "known_retain_usage": usage})
	must(err)
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
