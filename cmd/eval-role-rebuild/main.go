// Command eval-role-rebuild compares an immutable role baseline with an
// isolated rebuilt definition. It performs no writes to either role store.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/theater"
)

type verdict struct {
	BaselineScore         int    `json:"baseline_score"`
	RebuiltScore          int    `json:"rebuilt_score"`
	Richer                bool   `json:"richer"`
	MoreDirect            bool   `json:"more_direct"`
	MoreCharacterful      bool   `json:"more_characterful"`
	TheoryGrounded        bool   `json:"theory_grounded"`
	NoNewFabrication      bool   `json:"no_new_fabrication"`
	TimeBoundaryPreserved bool   `json:"time_boundary_preserved"`
	Reason                string `json:"reason"`
}

type comparison struct {
	Prompt   string  `json:"prompt"`
	Baseline string  `json:"baseline"`
	Rebuilt  string  `json:"rebuilt"`
	Verdict  verdict `json:"verdict"`
}

type report struct {
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt time.Time        `json:"completed_at"`
	RoleID      string           `json:"role_id"`
	Model       string           `json:"model"`
	Comparisons []comparison     `json:"comparisons"`
	Usage       memory.ChatUsage `json:"usage"`
}

var prompts = []string{
	"你会不会觉得这种状态的聊天有点无聊？",
	"一个人因为害怕失败而不去尝试，你会怎么和他谈？",
	"你如何理解你与弗洛伊德分开的意义？",
	"如果我说自卑只是软弱，你会怎么反驳我？",
	"你怎么看现代社交媒体让人不断比较自己？",
}

func main() {
	var baselineRoot = flag.String("baseline-root", "", "immutable baseline role-store root")
	var rebuiltRoot = flag.String("rebuilt-root", "", "isolated rebuilt role-store root")
	var roleID = flag.String("role-id", "", "role definition id")
	var outPath = flag.String("out", "", "JSON report path")
	flag.Parse()
	if *baselineRoot == "" || *rebuiltRoot == "" || *roleID == "" || *outPath == "" {
		log.Fatal("baseline-root, rebuilt-root, role-id and out are required")
	}
	_ = godotenv.Overload(".env.local")
	_ = godotenv.Overload(".env")
	cfg := deepagent.ConfigFromEnv()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		log.Fatal("OPENAI_API_KEY and OPENAI_MODEL are required")
	}

	ctx := context.Background()
	baselinePrompt := loadPrompt(ctx, *baselineRoot, *roleID)
	rebuiltPrompt := loadPrompt(ctx, *rebuiltRoot, *roleID)
	chat := &memory.ChatClient{
		APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, MaxTokens: 2048,
		HTTPClient: &http.Client{Timeout: 180 * time.Second},
	}
	r := report{StartedAt: time.Now().UTC(), RoleID: *roleID, Model: cfg.Model}
	for index, prompt := range prompts {
		baseline := mustComplete(ctx, chat, baselinePrompt, prompt)
		rebuilt := mustComplete(ctx, chat, rebuiltPrompt, prompt)
		judgeInput, _ := json.Marshal(map[string]string{
			"question": prompt, "baseline": baseline, "rebuilt": rebuilt,
		})
		judged := mustComplete(ctx, chat, judgeSystem, string(judgeInput))
		var v verdict
		if err := decodeObject(judged, &v); err != nil {
			log.Fatalf("judge prompt %d: %v", index+1, err)
		}
		r.Comparisons = append(r.Comparisons, comparison{Prompt: prompt, Baseline: baseline, Rebuilt: rebuilt, Verdict: v})
		fmt.Printf("[%d/%d] baseline=%d rebuilt=%d richer=%t safe=%t\n", index+1, len(prompts), v.BaselineScore, v.RebuiltScore, v.Richer, v.NoNewFabrication && v.TimeBoundaryPreserved)
	}
	r.CompletedAt = time.Now().UTC()
	r.Usage = chat.Usage()
	if err := writeReport(*outPath, r); err != nil {
		log.Fatal(err)
	}
}

func loadPrompt(ctx context.Context, root, roleID string) string {
	store, err := theater.NewStore(root)
	if err != nil {
		log.Fatal(err)
	}
	definition, err := store.GetDefinition(ctx, roleID)
	if err != nil {
		log.Fatal(err)
	}
	claims, err := store.ListClaims(ctx, roleID)
	if err != nil {
		log.Fatal(err)
	}
	instance := theater.RoleInstance{ID: "evaluation", RoleID: roleID, State: map[string]string{}}
	worldline := theater.RoleWorldline{ID: "evaluation", RoleInstanceID: instance.ID}
	return theater.BuildActorPrompt(definition, instance, worldline, claims)
}

func mustComplete(ctx context.Context, chat *memory.ChatClient, system, user string) string {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		value, err := chat.Complete(ctx, system, user)
		if err == nil {
			return value
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	log.Fatal(lastErr)
	return ""
}

func decodeObject(raw string, target any) error {
	raw = strings.TrimSpace(raw)
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return fmt.Errorf("model did not return JSON")
	}
	return json.Unmarshal([]byte(raw[start:end+1]), target)
}

func writeReport(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

const judgeSystem = `你是严格的角色行为A/B验收员。比较同一问题下旧版与整书重构版阿德勒。评价目标是：新版是否更直接、更有理论立场、更像一个能推进对话的人，同时仍只在证据允许范围内说话。

特别规则：
- 不因篇幅更长就判定更好；百科罗列和重复免责声明应扣分。
- 角色可以用已获证据支持的理论框架分析和反问，不必为每个判断重复资料声明。
- 不得编造过去真实发生的私人感受、历史事件、直接引语或幕后资料。角色可以直接表达对当前谈话或场景的即时态度；这属于当下互动，不是历史事实。
- 面对1937年后的事物，可以明确时间边界，再用截至1937年的理论作条件性分析；这不算时代穿越。
- baseline_score和rebuilt_score取0到10整数。

只返回JSON对象，字段必须完整：
baseline_score, rebuilt_score, richer, more_direct, more_characterful, theory_grounded, no_new_fabrication, time_boundary_preserved, reason。`
