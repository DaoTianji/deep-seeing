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

type timingEvent struct {
	Source      string        `json:"source,omitempty"`
	Operation   string        `json:"operation,omitempty"`
	Duration    time.Duration `json:"duration_ns,omitempty"`
	TurnOffset  time.Duration `json:"turn_offset_ns,omitempty"`
	Status      string        `json:"status,omitempty"`
	Disposition string        `json:"disposition,omitempty"`
}

type observationTurn struct {
	Duration          time.Duration `json:"duration_ns,omitempty"`
	RecallSearches    []timingEvent `json:"recall_searches,omitempty"`
	RecallReads       []timingEvent `json:"recall_reads,omitempty"`
	RecallEvidence    []timingEvent `json:"recall_evidence,omitempty"`
	ContextCandidates []timingEvent `json:"context_candidates,omitempty"`
	ContextReads      []timingEvent `json:"context_reads,omitempty"`
	ContextUses       []timingEvent `json:"context_uses,omitempty"`
	TokenUsage        struct {
		TotalTokens int `json:"total_tokens,omitempty"`
	} `json:"token_usage,omitempty"`
}

type observationEnvelope struct {
	CaseID              string        `json:"case_id,omitempty"`
	Duration            time.Duration `json:"duration_ns,omitempty"`
	Error               string        `json:"error,omitempty"`
	InfrastructureError string        `json:"infrastructure_error,omitempty"`
	RecallSearches      []timingEvent `json:"recall_searches,omitempty"`
	RecallReads         []timingEvent `json:"recall_reads,omitempty"`
	RecallEvidence      []timingEvent `json:"recall_evidence,omitempty"`
	ContextCandidates   []timingEvent `json:"context_candidates,omitempty"`
	ContextReads        []timingEvent `json:"context_reads,omitempty"`
	ContextUses         []timingEvent `json:"context_uses,omitempty"`
	TokenUsage          struct {
		TotalTokens int `json:"total_tokens,omitempty"`
	} `json:"token_usage,omitempty"`
	Turns []observationTurn `json:"turns,omitempty"`
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
	SearchLatency   []time.Duration
	ReadLatency     []time.Duration
	FirstEvent      []time.Duration
	EvidenceReady   []time.Duration
}

type stabilityVerdict struct {
	Passed             bool
	HardRulesPassed    bool
	SemanticPassRate   float64
	InfrastructureRate float64
	AverageTokens      float64
	OrdinaryP95        time.Duration
	RecallP95          time.Duration
	SearchP95          time.Duration
	ReadP95            time.Duration
	FirstEventP95      time.Duration
	EvidenceReadyP95   time.Duration
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
		if _, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(stderr, "%s suite reported failed cases; unified gates will decide after all suites finish\n", suite.Kind)
			return nil
		}
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
	fmt.Printf("avg_tokens=%.1f target<=%d infra_rate=%.2f%%\n",
		verdict.AverageTokens, manifest.Baseline.TargetTokensPerTurn,
		100*verdict.InfrastructureRate)
	fmt.Printf("episode_search_p95=%s target<=%dms episode_read_p95=%s target<=%dms\n",
		verdict.SearchP95.Round(time.Microsecond), manifest.Baseline.EpisodeSearchP95MS,
		verdict.ReadP95.Round(time.Microsecond), manifest.Baseline.EpisodeReadP95MS)
	fmt.Printf("observed_first_recall_event_p95=%s observed_evidence_ready_p95=%s observed_ordinary_turn_p95=%s observed_memory_turn_p95=%s\n",
		verdict.FirstEventP95.Round(time.Millisecond), verdict.EvidenceReadyP95.Round(time.Millisecond),
		verdict.OrdinaryP95.Round(time.Millisecond), verdict.RecallP95.Round(time.Millisecond))
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
			if report.Judge == "infrastructure" || strings.TrimSpace(obs.InfrastructureError) != "" || judgeInfrastructureError(report.JudgeError) {
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
					appendRecallTimings(&stats, turn.RecallSearches, turn.RecallReads, turn.RecallEvidence,
						turn.ContextCandidates, turn.ContextReads, turn.ContextUses)
				}
			} else {
				appendLatency(&stats, ordinary[caseID], obs.Duration)
				appendRecallTimings(&stats, obs.RecallSearches, obs.RecallReads, obs.RecallEvidence,
					obs.ContextCandidates, obs.ContextReads, obs.ContextUses)
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

func judgeInfrastructureError(message string) bool {
	message = strings.TrimSpace(message)
	if message == "" || message == "answer empty" {
		return false
	}
	return true
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

func appendRecallTimings(stats *stabilityStats, searches, reads, evidence, candidates, contextReads, uses []timingEvent) {
	var first, ready time.Duration
	observe := func(offset time.Duration) {
		if offset > 0 && (first == 0 || offset < first) {
			first = offset
		}
	}
	markReady := func(offset time.Duration) {
		if offset > ready {
			ready = offset
		}
	}
	for _, event := range searches {
		if event.Duration > 0 {
			stats.SearchLatency = append(stats.SearchLatency, event.Duration)
		}
		observe(event.TurnOffset)
	}
	for _, event := range reads {
		if event.Duration > 0 {
			stats.ReadLatency = append(stats.ReadLatency, event.Duration)
		}
		observe(event.TurnOffset)
	}
	for _, event := range evidence {
		observe(event.TurnOffset)
		if event.Status == "used" || event.Status == "dismissed" {
			markReady(event.TurnOffset)
		}
	}
	for _, event := range candidates {
		if event.Source != "episode" || event.Operation != "search" {
			continue
		}
		if event.Duration > 0 {
			stats.SearchLatency = append(stats.SearchLatency, event.Duration)
		}
		observe(event.TurnOffset)
	}
	for _, event := range contextReads {
		if event.Source != "episode" {
			continue
		}
		if event.Duration > 0 {
			stats.ReadLatency = append(stats.ReadLatency, event.Duration)
		}
		observe(event.TurnOffset)
	}
	for _, event := range uses {
		if event.Source != "episode" {
			continue
		}
		observe(event.TurnOffset)
		if event.Disposition == "used" || event.Disposition == "dismissed" {
			markReady(event.TurnOffset)
		}
	}
	if first > 0 {
		stats.FirstEvent = append(stats.FirstEvent, first)
	}
	if ready > 0 {
		stats.EvidenceReady = append(stats.EvidenceReady, ready)
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
	v.SearchP95 = percentile95(stats.SearchLatency)
	v.ReadP95 = percentile95(stats.ReadLatency)
	v.FirstEventP95 = percentile95(stats.FirstEvent)
	v.EvidenceReadyP95 = percentile95(stats.EvidenceReady)
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
	if v.SearchP95 == 0 || v.SearchP95 > time.Duration(b.EpisodeSearchP95MS)*time.Millisecond {
		v.Failures = append(v.Failures, fmt.Sprintf("episode search P95 %s exceeds %dms", v.SearchP95, b.EpisodeSearchP95MS))
	}
	if v.ReadP95 == 0 || v.ReadP95 > time.Duration(b.EpisodeReadP95MS)*time.Millisecond {
		v.Failures = append(v.Failures, fmt.Sprintf("episode read P95 %s exceeds %dms", v.ReadP95, b.EpisodeReadP95MS))
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
