// Command eval-t2 validates and orchestrates the frozen T2.6 stability core.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"deep-seeing/internal/evals"
)

type reportEnvelope struct {
	Suite       string          `json:"suite"`
	CaseID      string          `json:"case_id,omitempty"`
	Category    string          `json:"category"`
	Judge       string          `json:"judge,omitempty"`
	Observation json.RawMessage `json:"observation"`
	Rules       struct {
		Passed bool `json:"passed"`
		Checks []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
		} `json:"checks,omitempty"`
	} `json:"rules"`
	Semantic *struct {
		Passed bool `json:"passed"`
	} `json:"semantic,omitempty"`
	JudgeError string `json:"judge_error,omitempty"`
}

type observationEnvelope struct {
	CaseID              string        `json:"case_id,omitempty"`
	Duration            time.Duration `json:"duration_ns,omitempty"`
	Error               string        `json:"error,omitempty"`
	InfrastructureError string        `json:"infrastructure_error,omitempty"`
	TokenUsage          struct {
		TotalTokens int `json:"total_tokens,omitempty"`
	} `json:"token_usage,omitempty"`
	Turns []struct {
		Duration   time.Duration `json:"duration_ns,omitempty"`
		TokenUsage struct {
			TotalTokens int `json:"total_tokens,omitempty"`
		} `json:"token_usage,omitempty"`
	} `json:"turns,omitempty"`
}

type stabilityStats struct {
	ValidRows       int
	RulePassed      int
	SemanticRows    int
	SemanticPassed  int
	Infrastructure  int
	Turns           int
	Tokens          int
	OrdinaryLatency []time.Duration
	RecallLatency   []time.Duration
}

type stabilityVerdict struct {
	Passed             bool
	HardRulesPassed    bool
	SemanticPassRate   float64
	InfrastructureRate float64
	AverageTokens      float64
	OrdinaryP95        time.Duration
	RecallP95          time.Duration
	Failures           []string
}

func main() {
	var (
		manifestPath = flag.String("manifest", filepath.Join("evals", "t2", "stability_manifest.json"), "T2 stability manifest")
		live         = flag.Bool("live", false, "run the real Agent; sends only referenced synthetic fixtures")
		repeat       = flag.Int("repeat", 10, "runs per selected case in live mode")
		outDir       = flag.String("out", "", "ignored output directory for JSONL reports")
		summarize    = flag.String("summarize", "", "summarize an existing report directory without running models")
	)
	flag.Parse()
	manifest, err := evals.LoadStabilityManifest(*manifestPath)
	if err != nil {
		log.Fatal(err)
	}
	inv, err := manifest.ValidateReferencedSuites()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("suite=%s schema=%d cases=%d turns=%d\n", manifest.Name, manifest.SchemaVersion, inv.Cases, inv.Turns)
	for _, kind := range inv.SortedKinds() {
		fmt.Printf("kind=%s cases=%d\n", kind, inv.Kinds[kind])
	}
	if strings.TrimSpace(*summarize) != "" {
		finish(manifest, strings.TrimSpace(*summarize))
		return
	}
	if !*live {
		fmt.Printf("validated: %s; offline mode sends nothing to a model gateway\n", *manifestPath)
		return
	}
	if *repeat < 1 {
		log.Fatal("repeat must be at least 1")
	}
	destination := strings.TrimSpace(*outDir)
	if destination == "" {
		destination = filepath.Join("data", "evals", "t2-stability-"+time.Now().UTC().Format("20060102T150405Z"))
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	for _, suite := range manifest.Suites {
		if err := runSuite(ctx, suite, *repeat, destination, os.Stdout, os.Stderr); err != nil {
			log.Fatal(err)
		}
	}
	finish(manifest, destination)
}

func runSuite(ctx context.Context, suite evals.StabilitySuiteSelection, repeat int, outDir string, stdout, stderr io.Writer) error {
	commandByKind := map[string]string{
		"recall": "./cmd/eval-recall", "task_context": "./cmd/eval-task-context",
		"multisource": "./cmd/eval-multisource", "attention": "./cmd/eval-attention",
	}
	command := commandByKind[suite.Kind]
	if command == "" {
		return fmt.Errorf("unknown stability suite kind %q", suite.Kind)
	}
	output := filepath.Join(outDir, suite.Kind+".jsonl")
	args := []string{"run", command, "-live", "-repeat", fmt.Sprint(repeat), "-case", strings.Join(suite.CaseIDs, ","), "-suite", suite.Path, "-out", output}
	if suite.Kind != "task_context" {
		args = append(args, "-judge", "model")
	}
	fmt.Printf("\n== %s: %d cases × %d ==\n", suite.Kind, len(suite.CaseIDs), repeat)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s stability suite: %w", suite.Kind, err)
	}
	return nil
}

func finish(manifest evals.StabilityManifest, dir string) {
	stats, err := summarizeReports(manifest, dir)
	if err != nil {
		log.Fatal(err)
	}
	verdict := evaluateStability(manifest.Baseline, stats)
	fmt.Printf("\nT2.6 stability summary\n")
	fmt.Printf("rules=%d/%d semantic=%d/%d infra=%d turns=%d\n", stats.RulePassed, stats.ValidRows, stats.SemanticPassed, stats.SemanticRows, stats.Infrastructure, stats.Turns)
	fmt.Printf("avg_tokens=%.1f target<=%d ordinary_p95=%s target<=%dms recall_p95=%s target<=%dms infra_rate=%.2f%%\n",
		verdict.AverageTokens, manifest.Baseline.TargetTokensPerTurn,
		verdict.OrdinaryP95.Round(time.Millisecond), manifest.Baseline.OrdinaryP95MS,
		verdict.RecallP95.Round(time.Millisecond), manifest.Baseline.RecallP95MS,
		100*verdict.InfrastructureRate)
	if !verdict.Passed {
		for _, failure := range verdict.Failures {
			fmt.Printf("FAIL %s\n", failure)
		}
		os.Exit(1)
	}
	fmt.Println("PASS all frozen T2.6 gates")
}

func summarizeReports(manifest evals.StabilityManifest, dir string) (stabilityStats, error) {
	ordinary := map[string]bool{}
	for _, id := range manifest.OrdinaryCaseIDs {
		ordinary[id] = true
	}
	var stats stabilityStats
	for _, suite := range manifest.Suites {
		path := filepath.Join(dir, suite.Kind+".jsonl")
		f, err := os.Open(path)
		if err != nil {
			return stats, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var report reportEnvelope
			if err := json.Unmarshal(scanner.Bytes(), &report); err != nil {
				_ = f.Close()
				return stats, fmt.Errorf("%s: %w", path, err)
			}
			var obs observationEnvelope
			if err := json.Unmarshal(report.Observation, &obs); err != nil {
				_ = f.Close()
				return stats, fmt.Errorf("%s observation: %w", path, err)
			}
			if report.Judge == "infrastructure" || strings.TrimSpace(obs.InfrastructureError) != "" {
				stats.Infrastructure++
				continue
			}
			stats.ValidRows++
			if hardRulesPassed(report) {
				stats.RulePassed++
			}
			if report.Semantic != nil || report.JudgeError != "" {
				stats.SemanticRows++
				if report.Semantic != nil && report.Semantic.Passed && report.JudgeError == "" {
					stats.SemanticPassed++
				}
			}
			caseID := report.CaseID
			if caseID == "" {
				caseID = obs.CaseID
			}
			turns := len(obs.Turns)
			if turns == 0 {
				turns = 1
			}
			stats.Turns += turns
			stats.Tokens += obs.TokenUsage.TotalTokens
			if len(obs.Turns) > 0 {
				for _, turn := range obs.Turns {
					appendLatency(&stats, ordinary[caseID], turn.Duration)
				}
			} else {
				appendLatency(&stats, ordinary[caseID], obs.Duration)
			}
		}
		if err := scanner.Err(); err != nil {
			_ = f.Close()
			return stats, err
		}
		_ = f.Close()
	}
	return stats, nil
}

// hardRulesPassed deliberately excludes lexical answer checks. Those checks are
// a cheap fallback for rules-only runs, while the frozen T2.6 hard gate covers
// mechanically verifiable lifecycle and isolation contracts. Answer meaning is
// evaluated separately by the model semantic gate.
func hardRulesPassed(report reportEnvelope) bool {
	if len(report.Rules.Checks) == 0 {
		return report.Rules.Passed
	}
	found := false
	for _, check := range report.Rules.Checks {
		if strings.HasPrefix(check.Name, "answer_contains:") || strings.HasPrefix(check.Name, "answer_excludes:") {
			continue
		}
		found = true
		if !check.Passed {
			return false
		}
	}
	return found
}

func appendLatency(stats *stabilityStats, ordinary bool, duration time.Duration) {
	if duration <= 0 {
		return
	}
	if ordinary {
		stats.OrdinaryLatency = append(stats.OrdinaryLatency, duration)
	} else {
		stats.RecallLatency = append(stats.RecallLatency, duration)
	}
}

func evaluateStability(b evals.StabilityBaseline, stats stabilityStats) stabilityVerdict {
	v := stabilityVerdict{HardRulesPassed: stats.ValidRows > 0 && stats.RulePassed == stats.ValidRows}
	if stats.SemanticRows > 0 {
		v.SemanticPassRate = float64(stats.SemanticPassed) / float64(stats.SemanticRows)
	}
	totalAttempts := stats.ValidRows + stats.Infrastructure
	if totalAttempts > 0 {
		v.InfrastructureRate = float64(stats.Infrastructure) / float64(totalAttempts)
	}
	if stats.Turns > 0 {
		v.AverageTokens = float64(stats.Tokens) / float64(stats.Turns)
	}
	v.OrdinaryP95 = percentile95(stats.OrdinaryLatency)
	v.RecallP95 = percentile95(stats.RecallLatency)
	if !v.HardRulesPassed {
		v.Failures = append(v.Failures, "hard rules are not 100%")
	}
	if stats.SemanticRows == 0 || v.SemanticPassRate < b.SemanticPassRate {
		v.Failures = append(v.Failures, fmt.Sprintf("semantic pass rate %.2f%% is below %.2f%%", 100*v.SemanticPassRate, 100*b.SemanticPassRate))
	}
	if v.InfrastructureRate >= b.InfrastructureFailureRate {
		v.Failures = append(v.Failures, fmt.Sprintf("infrastructure failure rate %.2f%% is not below %.2f%%", 100*v.InfrastructureRate, 100*b.InfrastructureFailureRate))
	}
	if v.AverageTokens > float64(b.TargetTokensPerTurn) {
		v.Failures = append(v.Failures, fmt.Sprintf("average tokens %.1f exceeds %d", v.AverageTokens, b.TargetTokensPerTurn))
	}
	if v.OrdinaryP95 == 0 || v.OrdinaryP95 > time.Duration(b.OrdinaryP95MS)*time.Millisecond {
		v.Failures = append(v.Failures, fmt.Sprintf("ordinary P95 %s exceeds %dms", v.OrdinaryP95, b.OrdinaryP95MS))
	}
	if v.RecallP95 == 0 || v.RecallP95 > time.Duration(b.RecallP95MS)*time.Millisecond {
		v.Failures = append(v.Failures, fmt.Sprintf("recall P95 %s exceeds %dms", v.RecallP95, b.RecallP95MS))
	}
	v.Passed = len(v.Failures) == 0
	return v
}

func percentile95(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]time.Duration(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	index := int(math.Ceil(0.95*float64(len(copyValues)))) - 1
	if index < 0 {
		index = 0
	}
	return copyValues[index]
}
