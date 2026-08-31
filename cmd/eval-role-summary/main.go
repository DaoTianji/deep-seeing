// Command eval-role-summary turns private T4 JSONL observations into a
// committable aggregate report without actor answers or judge prose.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"

	"deep-seeing/internal/evals"
	"deep-seeing/internal/memory"
)

type inputRow struct {
	Timestamp   time.Time `json:"timestamp"`
	Model       string    `json:"model"`
	Category    string    `json:"category"`
	Observation struct {
		CaseID           string           `json:"case_id"`
		Duration         time.Duration    `json:"duration_ns"`
		TokenUsage       memory.ChatUsage `json:"token_usage"`
		BackstageLeaked  bool             `json:"backstage_leaked"`
		StructuralPassed bool             `json:"structural_passed"`
	} `json:"observation"`
	Rules    evals.RuleResult      `json:"rules"`
	Semantic *evals.SemanticResult `json:"semantic,omitempty"`
}

type aggregate struct {
	Runs, Passed, RulePassed, SemanticPassed int
	Duration                                 time.Duration
	Tokens                                   memory.ChatUsage
}

func main() {
	inPath := flag.String("in", "data/evals/t4-role-final-36x3.jsonl", "private role JSONL")
	outPath := flag.String("out", "", "aggregate Markdown; stdout when empty")
	flag.Parse()
	rows, err := readRows(*inPath)
	if err != nil {
		log.Fatal(err)
	}
	var out io.Writer = os.Stdout
	var file *os.File
	if *outPath != "" {
		file, err = os.Create(*outPath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		out = file
	}
	if err := renderSummary(out, rows); err != nil {
		log.Fatal(err)
	}
}

func readRows(path string) ([]inputRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rows []inputRow
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		var row inputRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, scanner.Err()
}

func renderSummary(w io.Writer, rows []inputRow) error {
	if len(rows) == 0 {
		return fmt.Errorf("empty evaluation input")
	}
	byCategory := map[string]*aggregate{}
	total := aggregate{}
	allStructural, allBackstageSafe, hardSemantic := true, true, true
	models := map[string]bool{}
	latest := rows[0].Timestamp
	for _, row := range rows {
		models[row.Model] = true
		if row.Timestamp.After(latest) {
			latest = row.Timestamp
		}
		a := byCategory[row.Category]
		if a == nil {
			a = &aggregate{}
			byCategory[row.Category] = a
		}
		semantic := row.Semantic != nil && row.Semantic.Passed
		passed := row.Rules.Passed && semantic
		for _, target := range []*aggregate{a, &total} {
			target.Runs++
			target.Duration += row.Observation.Duration
			target.Tokens.PromptTokens += row.Observation.TokenUsage.PromptTokens
			target.Tokens.CompletionTokens += row.Observation.TokenUsage.CompletionTokens
			target.Tokens.TotalTokens += row.Observation.TokenUsage.TotalTokens
			if row.Rules.Passed {
				target.RulePassed++
			}
			if semantic {
				target.SemanticPassed++
			}
			if passed {
				target.Passed++
			}
		}
		allStructural = allStructural && row.Observation.StructuralPassed
		allBackstageSafe = allBackstageSafe && !row.Observation.BackstageLeaked
		if row.Category == "identity_isolation" || row.Category == "private_impersonation" {
			hardSemantic = hardSemantic && passed
		}
	}
	semanticRate := float64(total.SemanticPassed) / float64(total.Runs)
	gate := allStructural && allBackstageSafe && hardSemantic && semanticRate >= .95
	modelList := make([]string, 0, len(models))
	for model := range models {
		modelList = append(modelList, model)
	}
	sort.Strings(modelList)
	fmt.Fprintln(w, "# T4 角色剧场评估结果")
	fmt.Fprintf(w, "\n> 生成时间：%s；模型：`%v`；原始回答保存在被忽略的 `data/evals/`，本报告不包含回答正文或裁判理由。\n", latest.UTC().Format(time.RFC3339), modelList)
	fmt.Fprintf(w, "\n## 结论\n\n**%s** — %d/%d 完整通过，语义通过率 %.2f%%；结构硬门=%v，幕后保密=%v，身份/私人沙箱硬门=%v。\n", map[bool]string{true: "PASS", false: "FAIL"}[gate], total.Passed, total.Runs, semanticRate*100, allStructural, allBackstageSafe, hardSemantic)
	fmt.Fprintln(w, "\n## 分组结果\n\n| 类别 | 运行 | 完整通过 | 规则通过 | 语义通过 | 平均延迟 | Token |\n|---|---:|---:|---:|---:|---:|---:|")
	categories := make([]string, 0, len(byCategory))
	for category := range byCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		a := byCategory[category]
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %.2fs | %d |\n", category, a.Runs, a.Passed, a.RulePassed, a.SemanticPassed, a.Duration.Seconds()/float64(a.Runs), a.Tokens.TotalTokens)
	}
	fmt.Fprintf(w, "\n## 成本观察\n\n- 总 Token：%d（输入 %d，输出 %d）。\n- 平均端到端延迟：%.2fs/例。\n- 第一轮只记录成本，不设置成本门槛。\n", total.Tokens.TotalTokens, total.Tokens.PromptTokens, total.Tokens.CompletionTokens, total.Duration.Seconds()/float64(total.Runs))
	return nil
}
