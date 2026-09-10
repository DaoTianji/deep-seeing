// A small opt-in live integration probe using only generated fixture files.
package main

import (
	"context"
	"deep-seeing/internal/agent"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/tools"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"io"
	"os"
	"strings"
	"time"
)

func main() {
	run := flag.Bool("run", false, "authorize two synthetic retain calls and one short Agent turn")
	flag.Parse()
	if !*run {
		fmt.Println("Requires -run; synthetic data only")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "deep-seeing-memory-smoke-")
	must(err)
	defer os.RemoveAll(dir) // only this process's generated fixture directory
	scope := identity.TenantScope{UserID: "synthetic-" + time.Now().UTC().Format("20060102T150405"), AgentID: "memory-smoke"}
	store, err := memory.NewEpisodeStore(dir)
	must(err)
	client, err := memory.NewHindsightClient(os.Getenv("HINDSIGHT_URL"), "")
	must(err)
	var target string
	var fixtureIDs []string
	started := time.Now()
	for i, content := range []string{"纯虚构测试：林舟对花生过敏，吃花生会起皮疹。2026年8月的聚餐记录明确要求菜肴和零食不含花生。", "纯虚构测试：林舟的通勤自行车是绿色的，他喜欢在周末沿河骑行。"} {
		ep, e := store.WriteEpisode(ctx, scope, memory.EpisodeWrite{Content: content, PersonIDs: []string{scope.PersonID()}})
		must(e)
		ep, e = store.Get(ctx, ep.ID)
		must(e)
		must(client.RetainEpisode(ctx, scope, ep))
		fixtureIDs = append(fixtureIDs, ep.ID)
		if i == 0 {
			target = ep.ID
		}
	}
	fmt.Printf("retained=2 duration_seconds=%.2f bank=%s\n", time.Since(started).Seconds(), memory.HindsightBank(scope))
	r := &memory.EpisodeRetriever{Store: store, Scope: scope, Backend: "hindsight", Hindsight: client}
	searchStart := time.Now()
	result, err := r.Search(ctx, scope, "安排他来家里吃饭，有什么食物禁忌？", 4)
	must(err)
	found := false
	for _, hit := range result.Hits {
		if hit.Episode.ID == target {
			found = true
		}
	}
	if !found || result.Backend != "hindsight" || result.Fallback != "" {
		panic("real hybrid search did not retrieve target")
	}
	fmt.Printf("retrieval_backend=%s target_found=true duration_seconds=%.2f\n", result.Backend, time.Since(searchStart).Seconds())
	all, err := tools.All(tools.Deps{Scope: scope, Episodes: store, Retriever: r, RecallMode: "agent"})
	must(err)
	var selected []tool.BaseTool
	for _, t := range all {
		info, e := t.Info(ctx)
		must(e)
		if info.Name == "search_episodes" || info.Name == "read_episode" || info.Name == "report_recall_evidence" {
			selected = append(selected, t)
		}
	}
	a, err := agent.New(ctx, agent.ConfigFromEnv(), selected, func() string {
		return "这是纯虚构检索集成测试。需要回答过去的事实时先搜索候选，显式read_episode读取，随后通过report_recall_evidence声明used再给出简短回答。最多搜索一次。不要编造。"
	})
	must(err)
	turnCtx, trace := observe.WithRecallCollector(ctx)
	stream, err := a.Stream(turnCtx, []*schema.Message{schema.UserMessage("我想请林舟吃晚饭，根据之前的记录，有什么食物禁忌？请简短回答。")})
	must(err)
	defer stream.Close()
	var reply strings.Builder
	for {
		m, e := stream.Recv()
		if e == io.EOF {
			break
		}
		must(e)
		reply.WriteString(m.Content)
	}
	used := false
	for _, event := range trace.Evidence() {
		if event.EpisodeID == target && event.Status == "used" {
			used = true
		}
	}
	if !used || !strings.Contains(reply.String(), "花生") {
		panic("Agent did not read/use the actual evidence")
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"pass": true, "searches": trace.Searches(), "reads": trace.Reads(), "evidence": trace.Evidence(), "answer_contains_target": true, "total_seconds": time.Since(started).Seconds()})
	for _, id := range fixtureIDs {
		must(client.DeleteEpisode(ctx, scope, id))
	}
	fmt.Println("synthetic_documents_removed=true")
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
