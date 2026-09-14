package story

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"deep-seeing/internal/memory"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type companionCrossCase struct {
	ID           string   `json:"id"`
	Book         string   `json:"book"`
	Speaker      string   `json:"speaker"`
	Evidence     string   `json:"evidence"`
	Anchor       string   `json:"anchor"`
	Message      string   `json:"message"`
	NeedsSpoiler bool     `json:"needs_spoiler"`
	Rubric       []string `json:"rubric"`
}

func companionCrossCases(t *testing.T) []companionCrossCase {
	t.Helper()
	b, err := os.ReadFile("testdata/companion-cross-book.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []companionCrossCase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func companionCrossFixture(t *testing.T, tc companionCrossCase) (*Engine, int) {
	t.Helper()
	for _, book := range Catalog() {
		if book.ID != tc.Book {
			continue
		}
		e := bookEngine(t, book)
		anchor := tc.Anchor
		if tc.Evidence != "" {
			anchor = book.Excerpts[tc.Evidence].Text
		}
		if anchor == "" || strings.Count(book.Text, anchor) != 1 {
			t.Fatalf("%s: anchor must exist exactly once", tc.ID)
		}
		end := strings.Index(book.Text, anchor) + len(anchor)
		for _, p := range e.Paragraphs() {
			if p.End < end {
				continue
			}
			if tc.Speaker != "an" && !slices.ContainsFunc(e.companionCharacters(p.ID), func(c Character) bool { return c.ID == tc.Speaker }) {
				t.Fatalf("%s: speaker not available at anchor", tc.ID)
			}
			return e, p.ID
		}
	}
	t.Fatalf("%s: missing book/paragraph", tc.ID)
	return nil, 0
}

// This preflight is offline: detect stale anchors before spending model budget.
func TestCompanionCrossBookFixtures(t *testing.T) {
	seen := map[string]bool{}
	books := map[string]int{}
	boundaries := map[string]int{}
	for _, tc := range companionCrossCases(t) {
		if seen[tc.ID] || tc.ID == "" || len(tc.Rubric) < 2 || tc.Message == "" {
			t.Fatalf("invalid case: %+v", tc)
		}
		seen[tc.ID] = true
		books[tc.Book]++
		if tc.NeedsSpoiler {
			boundaries[tc.Book]++
		}
		e, paragraph := companionCrossFixture(t, tc)
		if paragraph >= len(e.Paragraphs()) {
			t.Fatalf("%s: early-reading fixture reached the ending", tc.ID)
		}
	}
	for _, book := range Catalog() {
		if books[book.ID] != 2 || boundaries[book.ID] != 1 {
			t.Fatalf("%s needs perspective and boundary cases", book.ID)
		}
	}
}

// Explicit IDs and a three-case cap prevent accidental large paid runs. No
// automatic retries, no web research, and no access to existing reader stores.
// Structural success is NOT semantic acceptance: the report keeps human review
// pending and includes the fixed rubric plus public answers for inspection.
func TestCompanionCrossBookLive(t *testing.T) {
	envPath := os.Getenv("STORY_CROSS_BOOK_ENV")
	if envPath == "" {
		t.Skip("explicit live model opt-in required")
	}
	ids := strings.Split(os.Getenv("STORY_CROSS_BOOK_CASES"), ",")
	if len(ids) > 3 || ids[0] == "" {
		t.Fatal("choose 1–3 explicit case IDs")
	}
	reportPath := os.Getenv("STORY_CROSS_BOOK_REPORT")
	if reportPath == "" {
		t.Fatal("an ignored report path is required")
	}
	cases := companionCrossCases(t)
	selected := []companionCrossCase{}
	seen := map[string]bool{}
	for _, id := range ids {
		idx := slices.IndexFunc(cases, func(tc companionCrossCase) bool { return tc.ID == id })
		if idx < 0 || seen[id] {
			t.Fatalf("unknown/duplicate case ID: %s", id)
		}
		seen[id] = true
		companionCrossFixture(t, cases[idx])
		selected = append(selected, cases[idx])
	}
	env, err := godotenv.Read(envPath)
	if err != nil {
		t.Fatal("cannot read model configuration")
	}
	if env["OPENAI_API_KEY"] == "" || env["OPENAI_BASE_URL"] == "" || env["OPENAI_MODEL"] == "" {
		t.Fatal("model configuration is incomplete")
	}
	type result struct {
		Case             companionCrossCase `json:"case"`
		Model            string             `json:"model"`
		Paragraph        int                `json:"paragraph"`
		StructuralPassed bool               `json:"structural_passed"`
		SemanticReview   string             `json:"semantic_review"`
		Calls            int                `json:"calls"`
		Usage            memory.ChatUsage   `json:"usage"`
		ElapsedMS        int64              `json:"elapsed_ms"`
		Turn             *CompanionTurn     `json:"turn,omitempty"`
		FailureOutputs   []string           `json:"failure_public_outputs,omitempty"`
	}
	report := []result{}
	defer func() {
		if err := atomicJSON(reportPath, report); err != nil {
			t.Error(err)
		}
	}()
	for _, tc := range selected {
		t.Run(tc.ID, func(t *testing.T) {
			e, paragraph := companionCrossFixture(t, tc)
			c := &readingLiveClient{client: NewReadingGatewayClient(env["OPENAI_API_KEY"], env["OPENAI_BASE_URL"], env["OPENAI_MODEL"])}
			e.Chat, e.Model = c, c.client.Model
			owner, original := uuid.NewString(), e.textVersion()
			start := time.Now()
			r := result{Case: tc, Model: e.Model, Paragraph: paragraph, SemanticReview: "pending_human_review"}
			defer func() {
				r.StructuralPassed, r.Calls, r.Usage, r.ElapsedMS = !t.Failed(), c.calls, c.client.Usage(), time.Since(start).Milliseconds()
				if t.Failed() {
					r.FailureOutputs = c.responses
				}
				report = append(report, r)
				// Preserve completed cases even if a later call/process times out.
				if err := atomicJSON(reportPath, report); err != nil {
					t.Error(err)
				}
				t.Logf("structural_passed=%v calls=%d elapsed_ms=%d semantic_review=%s", r.StructuralPassed, r.Calls, r.ElapsedMS, r.SemanticReview)
			}()
			s, err := e.CompanionState(owner)
			if err != nil {
				t.Fatal(err)
			}
			s, err = e.UpdateReader(owner, ReaderPosition{Paragraph: paragraph}, s.Revision)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			s, err = e.CompanionTurn(ctx, owner, CompanionInput{RequestID: uuid.NewString(), Revision: s.Revision, Speaker: tc.Speaker, Paragraph: paragraph, Action: "chat", Message: tc.Message}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Turns) != 1 {
				t.Fatalf("expected one saved turn, got %d", len(s.Turns))
			}
			turn := s.Turns[0]
			r.Turn = &turn
			if turn.NeedsSpoiler != tc.NeedsSpoiler {
				t.Error("incorrect spoiler permission outcome")
			}
			if strings.TrimSpace(turn.Reply) == "" {
				t.Error("empty reply")
			}
			for _, p := range turn.Evidence {
				if p < 1 || p > paragraph {
					t.Error("evidence outside reading range")
				}
			}
			if e.textVersion() != original || len(e.Records(owner)) != 0 || s.Position.Full {
				t.Error("companion changed original/story/permission")
			}
			restored, err := e.CompanionState(owner)
			if err != nil || len(restored.Turns) != 1 || restored.Turns[0].Reply != turn.Reply {
				t.Error("saved answer did not restore")
			}
		})
	}
}
