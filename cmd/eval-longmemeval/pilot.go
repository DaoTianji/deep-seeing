package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/tools"
	"github.com/cloudwego/eino/components/tool"
)

// Labels select one item per stratum only; input() removes them before inference.
func pilotItems(path string) ([]Item, error) {
	seen := map[string]bool{}
	var xs []Item
	err := visit(path, func(x Item) error {
		if err := validate(x); err != nil {
			return err
		}
		if !seen[x.Type] {
			seen[x.Type] = true
			xs = append(xs, x)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(xs) != 6 {
		return nil, fmt.Errorf("pilot requires exactly six question categories")
	}
	return xs, nil
}
func pilotSave(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".pilot-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), path)
}
func pilotScope(x Input) identity.TenantScope {
	b, _ := json.Marshal(x)
	h := sha256.Sum256(b)
	return identity.TenantScope{UserID: "pilot-" + hex.EncodeToString(h[:16]), AgentID: "hindsight-pilot-v1"}
}

func hindsightPilot(data, out string) error {
	cfg := deepagent.ConfigFromEnv()
	if cfg.BaseURL != "http://127.0.0.1:8890/v1" || cfg.Model != "deepseek-ai/DeepSeek-V4-Pro" || cfg.APIKey == "" || os.Getenv("HINDSIGHT_URL") != "http://127.0.0.1:8891" {
		return fmt.Errorf("dedicated pilot proxy/service required")
	}
	if filepath.Base(filepath.Dir(out)) != "hindsight-pilot-50" {
		return fmt.Errorf("use independent hindsight-pilot-50 output directory")
	}
	xs, e := pilotItems(data)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(out), 0700); e != nil {
		return e
	}
	digest, e := hashFile(data)
	if e != nil {
		return e
	}
	ids := []string{}
	for _, x := range xs {
		ids = append(ids, x.ID)
	}
	manifest := map[string]any{"protocol": "hindsight-six-strata-pilot-v1", "dataset_sha256": digest, "ids": ids, "model": cfg.Model, "cap_cny": 50, "selection": "first item in each category, dataset order; no answer-based selection", "answer_output_cap": 8192, "retention": "full raw sessions; retained index; no retry uncertain writes", "judge": "separate official-prompt V4 Pro nonthinking, not official GPT-4o protocol"}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	mp := out + ".manifest.json"
	if old, e := os.ReadFile(mp); e == nil {
		if string(old) != string(mb) {
			return fmt.Errorf("pilot manifest mismatch")
		}
	} else if !os.IsNotExist(e) {
		return e
	} else if e = pilotSave(mp, manifest); e != nil {
		return e
	}
	for _, x := range xs {
		resultPath := filepath.Join(filepath.Dir(out), x.ID+".result.json")
		if _, e := os.Stat(resultPath); e == nil {
			continue
		}
		root := filepath.Join(filepath.Dir(out), "cases", x.ID)
		if e = os.MkdirAll(root, 0700); e != nil {
			return e
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		r, err := runHindsightPilot(ctx, cfg, x.input(), root)
		cancel()
		r.ID = x.ID
		r.Type = x.Type
		r.At = time.Now().UTC()
		r.Attempts = 1
		r.Error = safeError(err, cfg)
		if err != nil && r.Hypothesis == "" {
			_ = pilotSave(filepath.Join(root, "paused.json"), map[string]any{"error": r.Error, "at": r.At})
			return fmt.Errorf("pilot paused at %s: %s", x.ID, r.Error)
		}
		var candidates, reads, used []string
		for _, s := range r.Searches {
			candidates = append(candidates, s.ResultIDs...)
		}
		for _, s := range r.Reads {
			if s.Error == "" {
				reads = append(reads, s.EpisodeID)
			}
		}
		for _, s := range r.Evidence {
			if s.Status == "used" {
				used = append(used, s.EpisodeID)
			}
		}
		r.CandidateSessions = sessionIDs(candidates, r, x)
		r.ReadSessions = sessionIDs(reads, r, x)
		r.UsedSessions = sessionIDs(used, r, x)
		if e = pilotSave(resultPath, r); e != nil {
			return e
		}
		fmt.Printf("pilot completed id=%s category=%s seconds=%.1f searches=%d reads=%d\n", x.ID, x.Type, r.Seconds, len(r.Searches), len(r.Reads))
	}
	f, e := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	for _, x := range xs {
		b, e := os.ReadFile(filepath.Join(filepath.Dir(out), x.ID+".result.json"))
		if e != nil {
			return e
		}
		var r Result
		if e = json.Unmarshal(b, &r); e != nil {
			return e
		}
		if e = json.NewEncoder(f).Encode(r); e != nil {
			return e
		}
	}
	return f.Sync()
}

func runHindsightPilot(ctx context.Context, cfg deepagent.Config, x Input, root string) (Result, error) {
	r := Result{Mode: "hindsight-pilot", Model: cfg.Model, SessionMap: map[string]int{}}
	scope := pilotScope(x)
	store, e := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	if e != nil {
		return r, e
	}
	mapPath := filepath.Join(root, "sources.json")
	if b, e := os.ReadFile(mapPath); e == nil {
		if e = json.Unmarshal(b, &r.SessionMap); e != nil {
			return r, e
		}
	} else if !os.IsNotExist(e) {
		return r, e
	} else {
		prior, e := store.Snapshot(ctx)
		if e != nil {
			return r, e
		}
		if len(prior) > 0 {
			return r, fmt.Errorf("incomplete local import; inspect instead of duplicate")
		}
		order := make([]int, len(x.Sessions))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			a, _ := parseDate(x.Dates[order[i]])
			b, _ := parseDate(x.Dates[order[j]])
			return a.Before(b)
		})
		for _, i := range order {
			ep, e := store.WriteEpisode(ctx, scope, memory.EpisodeWrite{Kind: memory.EpisodeEvent, Content: sessionText(x.Dates[i], x.Sessions[i]), PersonIDs: []string{scope.PersonID()}, SessionID: fmt.Sprintf("session-%04d", i), Metadata: map[string]string{"historical_date": x.Dates[i]}})
			if e != nil {
				return r, e
			}
			r.SessionMap[ep.ID] = i
		}
		if e = pilotSave(mapPath, r.SessionMap); e != nil {
			return r, e
		}
	}
	if len(r.SessionMap) != len(x.Sessions) {
		return r, fmt.Errorf("source map incomplete")
	}
	c, e := memory.NewHindsightClient(os.Getenv("HINDSIGHT_URL"), "")
	if e != nil {
		return r, e
	}
	statuses := map[string]string{}
	jp := filepath.Join(root, "retain.json")
	if b, e := os.ReadFile(jp); e == nil {
		if e = json.Unmarshal(b, &statuses); e != nil {
			return r, e
		}
	} else if !os.IsNotExist(e) {
		return r, e
	}
	eps, e := store.Snapshot(ctx)
	if e != nil {
		return r, e
	}
	start := time.Now()
	for _, ep := range eps {
		if _, ok := r.SessionMap[ep.ID]; !ok {
			return r, fmt.Errorf("unexpected source")
		}
		if statuses[ep.ID] == "indexed" {
			continue
		}
		if statuses[ep.ID] != "" {
			return r, fmt.Errorf("uncertain retain; no automatic retry")
		}
		statuses[ep.ID] = "uncertain"
		if e = pilotSave(jp, statuses); e != nil {
			return r, e
		}
		if e = c.RetainEpisode(ctx, scope, ep); e != nil {
			return r, e
		}
		statuses[ep.ID] = "indexed"
		if e = pilotSave(jp, statuses); e != nil {
			return r, e
		}
		fmt.Printf("indexed bank=%s completed=%d total=%d\n", memory.HindsightBank(scope), len(statuses), len(eps))
	}
	r.IngestSeconds = time.Since(start).Seconds()
	retriever := &memory.EpisodeRetriever{Store: store, Scope: scope, Backend: "hindsight", Hindsight: c}
	all, e := tools.All(tools.Deps{Scope: scope, Episodes: store, Retriever: retriever, RecallMode: "agent", SessionID: "eval", Model: cfg.Model})
	if e != nil {
		return r, e
	}
	var selected []tool.BaseTool
	for _, v := range all {
		info, e := v.Info(ctx)
		if e != nil {
			return r, e
		}
		if info.Name == "search_episodes" || info.Name == "read_episode" || info.Name == "report_recall_evidence" {
			selected = append(selected, v)
		}
	}
	var service *runtime.Service
	a, e := deepagent.New(ctx, cfg, selected, func() string {
		if service == nil {
			return persona
		}
		return service.SystemProvider()
	})
	if e != nil {
		return r, e
	}
	service, e = runtime.New(runtime.Options{Scope: scope, SessionID: "eval", STM: memory.NewSTM(40), RecallMode: runtime.RecallModeAgent, Agent: a, Soul: persona, Model: cfg.Model, Assembler: benchAssembler{}, Capability: "Available tools: search_episodes, read_episode, report_recall_evidence. Each Episode contains one historical conversation session. No external tools or persistent write tools are available."})
	if e != nil {
		return r, e
	}
	answerCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	start = time.Now()
	var mu sync.Mutex
	out, e := service.StreamTurnWithHooks(answerCtx, message(x), runtime.TurnHooks{OnToolStart: func(s string) { mu.Lock(); defer mu.Unlock(); r.Calls = append(r.Calls, s) }, OnRecallSearch: func(s observe.RecallSearchTrace) { mu.Lock(); defer mu.Unlock(); r.Searches = append(r.Searches, s) }, OnRecallRead: func(s observe.RecallReadTrace) { mu.Lock(); defer mu.Unlock(); r.Reads = append(r.Reads, s) }, OnRecallEvidence: func(s observe.RecallEvidenceTrace) { mu.Lock(); defer mu.Unlock(); r.Evidence = append(r.Evidence, s) }})
	r.Seconds = time.Since(start).Seconds()
	r.Hypothesis = out.Answer
	r.Tokens = out.TokenUsage
	for _, s := range r.Searches {
		if s.Backend != "hindsight" || s.Fallback != "" {
			return r, fmt.Errorf("non-Hindsight retrieval invalidates pilot")
		}
	}
	return r, e
}
