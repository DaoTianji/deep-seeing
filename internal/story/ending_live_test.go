package story

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"deep-seeing/internal/memory"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// Re-audit an explicitly selected synthetic saved card, without rerunning the
// story or modifying its signed artifact. The observed bad fixture contains an
// extra completed payment that is absent from the committed events.
func TestStoryLiveEndingCardAudit(t *testing.T) {
	envPath, fixture := os.Getenv("STORY_ENDING_AUDIT_ENV"), os.Getenv("STORY_ENDING_AUDIT_FIXTURE")
	if envPath == "" || fixture == "" {
		t.Skip("explicit model and synthetic card opt-in required")
	}
	env, err := godotenv.Read(envPath)
	if err != nil {
		t.Fatal("cannot read environment")
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal("cannot read selected fixture")
	}
	var b Branch
	if err = json.Unmarshal(raw, &b); err != nil || b.Ending == nil {
		t.Fatal("expected completed synthetic branch")
	}
	original := b.Ending
	c := &readingLiveClient{client: &memory.ChatClient{APIKey: env["OPENAI_API_KEY"], BaseURL: env["OPENAI_BASE_URL"], Model: env["OPENAI_MODEL"], MaxTokens: 8192, Thinking: "enabled", ReasoningEffort: "low", HTTPClient: &http.Client{Timeout: 150 * time.Second}}}
	e, err := New(t.TempDir(), c, c.client.Model, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if b.BookVersion != e.Book.Version {
		t.Fatal("unsupported fixture book")
	}
	d := Decision{Ending: original, NextScene: &b.Scene}
	b.Ending = nil // Candidate prose never goes into the authority.
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	start := time.Now()
	result, verdict, runErr := e.reviewEnding(ctx, b, d, uuid.NewString(), "")
	passed := runErr == nil && c.calls == 3 && result.Ending.Story != original.Story
	report := map[string]any{"passed": passed, "calls": c.calls, "usage": c.client.Usage(), "elapsed_ms": time.Since(start).Milliseconds(), "verdict": verdict, "ending": result.Ending, "public_model_outputs": c.responses}
	if runErr != nil {
		report["error"] = runErr.Error()
	}
	if path := os.Getenv("STORY_ENDING_AUDIT_REPORT"); path != "" {
		if err := atomicJSON(path, report); err != nil {
			t.Fatal("cannot save report")
		}
	}
	if !passed {
		t.Fatalf("known extra payment not corrected and approved: calls=%d err=%v", c.calls, runErr)
	}
	t.Logf("corrected without changing fixture calls=%d elapsed=%s", c.calls, time.Since(start))
}

// Diagnose a specifically opted-in, synthetic browser fixture in temporary
// storage. Never writes the input fixture or retries the outer turn. The
// report retains public model output (not reasoning_content) so a rejection
// can be explained without turning speculative content into story memory.
func TestStoryLiveFinishFixture(t *testing.T) {
	envPath, fixture := os.Getenv("STORY_FINISH_LIVE_ENV"), os.Getenv("STORY_FINISH_LIVE_FIXTURE")
	if envPath == "" || fixture == "" {
		t.Skip("explicit model and synthetic fixture opt-in required")
	}
	env, err := godotenv.Read(envPath)
	if err != nil {
		t.Fatal("cannot read live environment")
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal("cannot read opted-in fixture")
	}
	var b Branch
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal("invalid branch fixture")
	}
	c := &readingLiveClient{client: &memory.ChatClient{APIKey: env["OPENAI_API_KEY"], BaseURL: env["OPENAI_BASE_URL"], Model: env["OPENAI_MODEL"], MaxTokens: 8192, Thinking: "enabled", ReasoningEffort: "low", HTTPClient: &http.Client{Timeout: 150 * time.Second}}}
	e, err := New(t.TempDir(), c, c.client.Model, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if b.BookVersion != e.Book.Version || b.Ending != nil {
		t.Fatal("fixture must be an unfinished current necklace branch")
	}
	b.Owner, b.ID = uuid.NewString(), uuid.NewString()
	if err := e.save(b); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	start := time.Now()
	out, runErr := e.TurnWithOptions(ctx, b.Owner, b.ID, "loisel", "", uuid.NewString(), b.Revision, false, true, b.Influence)
	report := map[string]any{"calls": c.calls, "usage": c.client.Usage(), "elapsed_ms": time.Since(start).Milliseconds(), "passed": runErr == nil, "public_model_outputs": c.responses, "branch": out}
	if runErr != nil {
		report["error"] = runErr.Error()
	}
	if path := os.Getenv("STORY_FINISH_LIVE_REPORT"); path != "" {
		if err := atomicJSON(path, report); err != nil {
			t.Fatal("could not save diagnostic report")
		}
	}
	if runErr != nil {
		got, _ := e.Get(b.Owner, b.ID)
		if got.Revision != b.Revision || got.Ending != nil {
			t.Error("rejected ending changed the story")
		}
		t.Fatal(runErr)
	}
	if out.Ending == nil {
		t.Fatal("missing ending")
	}
	t.Logf("finished calls=%d elapsed=%s", c.calls, time.Since(start))
}
