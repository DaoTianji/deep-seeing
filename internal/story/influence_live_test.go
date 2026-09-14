package story

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type influenceCase struct {
	ID        string   `json:"id"`
	Scene     int      `json:"scene"`
	Character string   `json:"character"`
	Influence int      `json:"influence"`
	Message   string   `json:"message"`
	Rubric    []string `json:"rubric"`
}

func influenceCases(t *testing.T) []influenceCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/influence-comparison.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []influenceCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestInfluenceComparisonFixtures(t *testing.T) {
	cases := influenceCases(t)
	if len(cases) != 3 || cases[0].Influence != 5 || cases[1].Influence != 10 || cases[2].Influence != 10 {
		t.Fatal("need a same-advice 5/10 pair and opposite-advice 10 control")
	}
	if cases[0].Message != cases[1].Message || cases[1].Message == cases[2].Message {
		t.Fatal("comparison changed more than the intended variable")
	}
	e, _, owner, _ := setup(t)
	seen := map[string]bool{}
	for _, tc := range cases {
		if seen[tc.ID] || tc.ID == "" || len(tc.Rubric) < 3 || tc.Scene != 4 || tc.Character != "mathilde" {
			t.Fatal("invalid controlled fixture")
		}
		seen[tc.ID] = true
		b, err := e.Create(owner, tc.Scene)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Snapshot(b, tc.Character); err != nil {
			t.Fatal(err)
		}
	}
}

type cappedInfluenceClient struct {
	*readingLiveClient
	limit int
	logf  func(string, ...any)
}

func validateInfluenceResume(tc, saved influenceCase, b Branch) error {
	if !reflect.DeepEqual(tc, saved) || b.Ending != nil || b.Revision != 2 || b.Anchor != tc.Scene || b.Influence != tc.Influence || len(b.Turns) != 1 {
		return fmt.Errorf("resume must be the exact fixed case after its first successful advice")
	}
	t := b.Turns[0]
	if t.Kind != "chat" || t.CharacterID != tc.Character || t.Message != tc.Message || t.Influence != tc.Influence {
		return fmt.Errorf("resume advice differs from fixed case")
	}
	return nil
}

func TestInfluenceResumeKeepsOriginalAdvice(t *testing.T) {
	tc := influenceCases(t)[1]
	b := Branch{Anchor: tc.Scene, Revision: 2, Influence: tc.Influence, Turns: []Turn{{Kind: "chat", CharacterID: tc.Character, Message: tc.Message, Influence: tc.Influence}}}
	if err := validateInfluenceResume(tc, tc, b); err != nil {
		t.Fatal(err)
	}
	changed := tc
	changed.Message = "different advice"
	if validateInfluenceResume(tc, changed, b) == nil {
		t.Fatal("accepted a different experimental condition")
	}
	b.Ending = &Ending{Title: "already finished"}
	if validateInfluenceResume(tc, tc, b) == nil {
		t.Fatal("reran a completed sample")
	}
	b.Ending = nil
	b.Turns[0].Influence = 5
	if validateInfluenceResume(tc, tc, b) == nil {
		t.Fatal("changed influence while resuming")
	}
	c := &cappedInfluenceClient{readingLiveClient: &readingLiveClient{}, limit: 0}
	if _, err := c.Complete(context.Background(), "", ""); err == nil || c.calls != 0 {
		t.Fatal("budget did not reject before contacting a model")
	}
}

func (c *cappedInfluenceClient) Complete(ctx context.Context, system, input string) (string, error) {
	if c.calls >= c.limit {
		return "", fmt.Errorf("explicit live-test model call budget exhausted")
	}
	started := time.Now()
	if c.logf != nil {
		c.logf("model call %d/%d started", c.calls+1, c.limit)
	}
	out, err := c.readingLiveClient.Complete(ctx, system, input)
	if c.logf != nil {
		c.logf("model call %d/%d finished elapsed=%s error=%t", c.calls, c.limit, time.Since(started), err != nil)
	}
	return out, err
}

// Exactly one explicitly selected case per invocation, at most 8 model calls,
// one private conversation then one finish, no outer retries. The fixed 3-case
// comparison is exploratory: successful commits do not prove semantic quality
// or that every high-influence sample must differ from every low sample.
func TestStoryInfluenceLiveComparison(t *testing.T) {
	path := os.Getenv("STORY_INFLUENCE_ENV")
	if path == "" {
		t.Skip("explicit live opt-in required")
	}
	var tc influenceCase
	for _, candidate := range influenceCases(t) {
		if candidate.ID == os.Getenv("STORY_INFLUENCE_CASE") {
			tc = candidate
		}
	}
	output := os.Getenv("STORY_INFLUENCE_REPORT")
	if tc.ID == "" || output == "" {
		t.Fatal("select one fixed case and an ignored report path")
	}
	env, err := godotenv.Read(path)
	if err != nil {
		t.Fatal("cannot read model configuration")
	}
	c := &cappedInfluenceClient{readingLiveClient: &readingLiveClient{client: NewReadingGatewayClient(env["OPENAI_API_KEY"], env["OPENAI_BASE_URL"], env["OPENAI_MODEL"])}, limit: 8, logf: t.Logf}
	e, err := New(t.TempDir(), c, c.client.Model, "agent")
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	b, err := e.Create(owner, tc.Scene)
	if err != nil {
		t.Fatal(err)
	}
	original := e.textVersion()
	start := time.Now()
	report := map[string]any{"case": tc, "model": c.client.Model, "structural_passed": false, "semantic_review": "pending_human_comparison", "initial_branch": b}
	steps := []bool{false, true}
	if resume := os.Getenv("STORY_INFLUENCE_RESUME_REPORT"); resume != "" {
		if filepath.Clean(resume) == filepath.Clean(output) {
			t.Fatal("never overwrite the original observed report")
		}
		raw, err := os.ReadFile(resume)
		if err != nil {
			t.Fatal("cannot read explicitly selected synthetic report")
		}
		var saved struct {
			Case   influenceCase `json:"case"`
			Model  string        `json:"model"`
			Advice Branch        `json:"advice_branch"`
		}
		if json.Unmarshal(raw, &saved) != nil || saved.Model != c.client.Model || saved.Advice.BookVersion != e.Book.Version {
			t.Fatal("resume report model/book mismatch")
		}
		if err := validateInfluenceResume(tc, saved.Case, saved.Advice); err != nil {
			t.Fatal(err)
		}
		b = saved.Advice
		b.Owner = owner
		if err := e.save(b); err != nil {
			t.Fatal(err)
		}
		report["resumed_from"], report["advice_branch"] = resume, b
		steps, c.limit = []bool{true}, 5
	}
	defer func() {
		report["calls"], report["usage"], report["elapsed_ms"] = c.calls, c.client.Usage(), time.Since(start).Milliseconds()
		report["public_model_outputs"] = c.responses
		if err := atomicJSON(output, report); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for _, finish := range steps {
		if finish && b.Ending != nil {
			report["automatic_ending"] = true
			break
		}
		kind := "advice"
		if finish {
			kind = "finish"
		}
		t.Logf("case=%s step=%s started", tc.ID, kind)
		before := b.Revision
		beforeScene := b.Scene
		out, err := e.TurnWithOptions(ctx, owner, b.ID, tc.Character, tc.Message, uuid.NewString(), b.Revision, false, finish, tc.Influence)
		if err != nil {
			report["failed_step"], report["error"] = kind, err.Error()
			// Preserve the actual rejection before temporary storage is removed;
			// never run another model request merely to recover its reason.
			if raw, readErr := os.ReadFile(filepath.Join(e.Root, owner, "diagnostics", b.ID+".json")); readErr == nil {
				var diagnostic turnFailure
				if json.Unmarshal(raw, &diagnostic) == nil {
					report["failure_diagnostic"] = diagnostic
				}
			}
			got, loadErr := e.Get(owner, b.ID)
			report["last_committed_branch"] = got
			if loadErr != nil || got.Revision != before {
				t.Error("failed turn changed stored revision")
			}
			t.Fatal(err)
		}
		b = out
		report[kind+"_branch"] = b
		if err := atomicJSON(output, report); err != nil {
			t.Fatal(err)
		}
		if b.Influence != tc.Influence || b.Revision != before+1 {
			t.Fatal("influence/revision not actually committed")
		}
		if !finish {
			// A recorded decision is not a completed visit. Check structural
			// isolation here; actual event meaning remains part of human review.
			if !reflect.DeepEqual(b.Scene, beforeScene) {
				t.Fatal("private advice advanced the shared scene")
			}
			for _, m := range b.Memories {
				if m.CharacterID != tc.Character {
					t.Fatal("private advice updated another person's memory")
				}
			}
		}
		t.Logf("case=%s step=%s saved revision=%d", tc.ID, kind, b.Revision)
	}
	if e.textVersion() != original || b.Ending == nil {
		t.Fatal("original changed or ending missing")
	}
	reloaded, err := e.Get(owner, b.ID)
	if err != nil || reloaded.Ending == nil || reloaded.Revision != b.Revision {
		t.Fatal("ending not persisted")
	}
	report["structural_passed"] = true
}
