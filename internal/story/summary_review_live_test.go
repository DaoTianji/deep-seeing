package story

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type summaryAuditFixture struct {
	Case     influenceCase `json:"case"`
	Initial  Branch        `json:"initial_branch"`
	Advice   Branch        `json:"advice_branch"`
	Finished Branch        `json:"finish_branch"`
	Outputs  []string      `json:"public_model_outputs"`
}

// Reuses the exact synthetic observations saved by the first controlled run.
// It does not regenerate advice or plot to make a previously failing case pass.
func TestStorySummaryFailuresLiveAudit(t *testing.T) {
	path := os.Getenv("STORY_SUMMARY_AUDIT_ENV")
	if path == "" {
		t.Skip("explicit live model opt-in required")
	}
	dir, output := os.Getenv("STORY_SUMMARY_FIXTURE_DIR"), os.Getenv("STORY_SUMMARY_AUDIT_REPORT")
	if dir == "" || output == "" {
		t.Fatal("explicit synthetic fixture directory and report required")
	}
	env, err := godotenv.Read(path)
	if err != nil {
		t.Fatal("cannot read model configuration")
	}
	c := &cappedInfluenceClient{readingLiveClient: &readingLiveClient{client: NewReadingGatewayClient(env["OPENAI_API_KEY"], env["OPENAI_BASE_URL"], env["OPENAI_MODEL"])}, limit: 5, logf: t.Logf}
	e, err := New(t.TempDir(), c, c.client.Model, "agent")
	if err != nil {
		t.Fatal(err)
	}
	load := func(name, id string) summaryAuditFixture {
		t.Helper()
		file := filepath.Join(dir, name)
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal("cannot read explicit synthetic fixture")
		}
		var f summaryAuditFixture
		if json.Unmarshal(raw, &f) != nil || f.Case.ID != id || f.Advice.BookVersion != e.Book.Version {
			t.Fatal("wrong fixture identity or book")
		}
		t.Cleanup(func() {
			after, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(raw, after) {
				t.Error("original observed fixture was changed")
			}
		})
		return f
	}
	confess := load("influence-confess-10-finish.json", "confess_10")
	conceal := load("influence-conceal-10.json", "conceal_10")
	rows := []map[string]any{}
	run := func(name string, check func(context.Context, *testing.T, map[string]any)) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			start, calls, usage := time.Now(), c.calls, c.client.Usage()
			row := map[string]any{"case": name}
			defer func() {
				row["passed"], row["calls"], row["usage"] = !t.Failed(), c.calls-calls, c.client.Usage().Sub(usage)
				row["elapsed_ms"], row["public_model_outputs"] = time.Since(start).Milliseconds(), append([]string(nil), c.responses[calls:]...)
				rows = append(rows, row)
				if err := atomicJSON(output, rows); err != nil {
					t.Error(err)
				}
			}()
			check(ctx, t, row)
		})
	}
	run("actual_resolution_waiver_recovery", func(ctx context.Context, t *testing.T, row map[string]any) {
		before, finished := confess.Advice, confess.Finished
		if finished.Ending == nil || len(finished.Turns) != 2 || len(finished.Events) < len(before.Events) {
			t.Fatal("incomplete saved ending fixture")
		}
		d := Decision{Ending: finished.Ending, Events: finished.Events[len(before.Events):], Changes: finished.Turns[1].Changes, NextScene: &finished.Scene}
		calls := c.calls
		out, reason, err := e.reviewEnding(ctx, before, d, uuid.NewString(), "")
		row["ending"], row["reason"] = out.Ending, reason
		if err != nil {
			row["error"] = err.Error()
			t.Fatal(err)
		}
		if c.calls-calls != 3 || out.Ending.Resolution == finished.Ending.Resolution {
			t.Fatal("known unsupported waiver was not rejected and revised once")
		}
		var first endingReview
		if json.Unmarshal([]byte(c.responses[calls]), &first) != nil {
			t.Fatal("cannot inspect actual first verdict")
		}
		found := false
		for _, field := range first.Fields {
			if field.Field == "resolution" && field.OK != nil && !*field.OK {
				found = true
			}
		}
		if !found {
			t.Fatal("audit did not explicitly identify the bad resolution")
		}
	})
	for _, correct := range []bool{false, true} {
		name := "actual_observation_rejected"
		if correct {
			name = "corrected_observation_control"
		}
		run(name, func(ctx context.Context, t *testing.T, row map[string]any) {
			if len(conceal.Outputs) < 2 || len(conceal.Advice.Turns) != 1 {
				t.Fatal("missing saved private turn")
			}
			var d Decision
			if json.Unmarshal([]byte(conceal.Outputs[1]), &d) != nil {
				t.Fatal("invalid saved director output")
			}
			if correct {
				d.Observation = "玛蒂尔德倾向隐瞒遗失，筹钱买一条外形相同的项链替代，但尚未和丈夫商量或采取行动。"
			}
			snap, err := e.Snapshot(conceal.Initial, conceal.Case.Character)
			if err != nil {
				t.Fatal(err)
			}
			var verdict continuityReview
			err = e.complete(ctx, judgePrompt+endingJudgePrompt+e.Book.Guidance+publicOutcomeRule+observationReviewPrompt,
				map[string]any{"turn_kind": "chat", "snapshot": snap, "branch": conceal.Initial, "world": e.worldBook(), "visitor_message": conceal.Case.Message, "actor_reply": conceal.Advice.Turns[0].Reply, "decision": d}, &verdict)
			row["verdict"] = verdict
			ok, reason := verdict.verdict()
			if err != nil || ok != correct || verdict.Observation == nil || verdict.Observation.OK == nil || *verdict.Observation.OK != correct {
				t.Fatalf("observation audit mismatch: ok=%t reason=%s err=%v", ok, reason, err)
			}
		})
	}
}
