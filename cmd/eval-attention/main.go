package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"deep-seeing/internal/evals"
)

func main() {
	var (
		suitePath = flag.String("suite", filepath.Join("evals", "t2", "attention_cases.json"), "attention suite JSON")
		live      = flag.Bool("live", false, "run the real Agent in isolated synthetic sandboxes")
		repeat    = flag.Int("repeat", 1, "runs per case")
		caseID    = flag.String("case", "", "run one case ID")
		judge     = flag.String("judge", "rules", "rules or model")
		outPath   = flag.String("out", "", "optional ignored JSONL report path")
		timeout   = flag.Duration("timeout", 12*time.Minute, "timeout per multi-turn case")
	)
	flag.Parse()
	suite, err := evals.LoadAttentionSuite(*suitePath)
	if err != nil {
		log.Fatal(err)
	}
	selected := selectAttentionCases(suite.Cases, strings.TrimSpace(*caseID))
	if len(selected) == 0 {
		log.Fatalf("unknown case %q", *caseID)
	}
	if *repeat < 1 {
		log.Fatal("repeat must be at least 1")
	}
	if *judge != "rules" && *judge != "model" {
		log.Fatal("judge must be rules or model")
	}
	categories := map[string]int{}
	turns := 0
	for _, c := range selected {
		categories[c.Category]++
		turns += len(c.Turns)
	}
	keys := make([]string, 0, len(categories))
	for category := range categories {
		keys = append(keys, category)
	}
	sort.Strings(keys)
	fmt.Printf("suite=%s schema=%d cases=%d selected=%d turns=%d\n", suite.Name, suite.SchemaVersion, len(suite.Cases), len(selected), turns)
	for _, category := range keys {
		fmt.Printf("category=%s cases=%d\n", category, categories[category])
	}
	if !*live {
		fmt.Printf("validated: %s; this command is offline and sends nothing to a model gateway\n", *suitePath)
		return
	}
	if err := runAttentionLive(suite, selected, attentionLiveOptions{
		Repeat: *repeat, Judge: *judge, OutPath: strings.TrimSpace(*outPath), Timeout: *timeout,
	}); err != nil {
		log.Fatal(err)
	}
}
