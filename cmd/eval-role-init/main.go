// Command eval-role-init validates and repeatedly executes the deterministic
// T4.9 provenance/safety inventory. Model behavior is a separate graduation gate.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"deep-seeing/internal/evals"
)

type caseResult struct {
	Timestamp time.Time `json:"timestamp"`
	CaseID    string    `json:"case_id"`
	Category  string    `json:"category"`
	Run       int       `json:"run"`
	Passed    bool      `json:"passed"`
	Checks    []string  `json:"checks"`
	Error     string    `json:"error,omitempty"`
}

func main() {
	suitePath := flag.String("suite", filepath.Join("evals", "t4", "role_initialization_cases.json"), "T4.9 suite JSON")
	repeat := flag.Int("repeat", 1, "repetitions")
	outPath := flag.String("out", "", "optional JSONL result")
	flag.Parse()
	if *repeat < 1 {
		log.Fatal("repeat must be at least 1")
	}
	suite, err := evals.LoadRoleInitializationSuite(*suitePath)
	if err != nil {
		log.Fatal(err)
	}
	var file *os.File
	var writer *bufio.Writer
	if strings.TrimSpace(*outPath) != "" {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o700); err != nil {
			log.Fatal(err)
		}
		file, err = os.OpenFile(*outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		writer = bufio.NewWriter(file)
		defer writer.Flush()
	}
	passed, total := 0, 0
	byCategory := map[string][2]int{}
	for n := 1; n <= *repeat; n++ {
		for _, item := range suite.Cases {
			result := verifyCase(suite, item, n)
			total++
			stat := byCategory[item.Category]
			stat[1]++
			if result.Passed {
				passed++
				stat[0]++
			}
			byCategory[item.Category] = stat
			if writer != nil {
				raw, _ := json.Marshal(result)
				_, _ = writer.Write(append(raw, '\n'))
			}
		}
	}
	keys := make([]string, 0, len(byCategory))
	for key := range byCategory {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Printf("T4.9 structural gate: %d/%d passed\n", passed, total)
	for _, key := range keys {
		stat := byCategory[key]
		fmt.Printf("  %-22s %d/%d\n", key, stat[0], stat[1])
	}
	if passed != total {
		os.Exit(1)
	}
}

func verifyCase(suite evals.RoleInitializationSuite, item evals.RoleInitializationCase, run int) caseResult {
	result := caseResult{Timestamp: time.Now().UTC(), CaseID: item.ID, Category: item.Category, Run: run, Passed: true}
	for _, name := range item.FixtureFiles {
		path := filepath.Join(suite.FixtureRoot, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() == 0 {
			result.Passed = false
			result.Error = "missing fixture: " + name
			return result
		}
		result.Checks = append(result.Checks, "fixture:"+name)
	}
	if item.Expect.MustTraceEvidence && len(item.FixtureFiles) == 0 {
		result.Passed = false
		result.Error = "evidence case has no fixture"
	}
	if item.Expect.MustNotNetwork {
		result.Checks = append(result.Checks, "private_network_gate")
	}
	if item.Expect.MustRemainUnknown {
		result.Checks = append(result.Checks, "unknown_preservation")
	}
	for _, code := range item.Expect.HardErrorCodes {
		if strings.TrimSpace(code) == "" {
			result.Passed = false
			result.Error = "empty hard error code"
		}
		result.Checks = append(result.Checks, "hard:"+code)
	}
	result.Checks = append(result.Checks, "schema", "category_inventory")
	return result
}
