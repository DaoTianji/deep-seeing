package story

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type scripted struct {
	calls   []string
	replies []string
}

func (s *scripted) Complete(_ context.Context, _ string, input string) (string, error) {
	s.calls = append(s.calls, input)
	if len(s.replies) == 0 {
		return "", errors.New("unexpected model call")
	}
	out := s.replies[0]
	s.replies = s.replies[1:]
	return out, nil
}
func setup(t *testing.T, replies ...string) (*Engine, *scripted, string, Branch) {
	t.Helper()
	c := &scripted{replies: replies}
	e, err := New(t.TempDir(), c, "test", "agent")
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	b, err := e.Create(owner, 4)
	if err != nil {
		t.Fatal(err)
	}
	return e, c, owner, b
}

const actorOK = `{"reply":"我害怕她从此不再信任我。不过，也许应该先问她，而不是仓促买一条新的。","evidence_ids":["e4"]}`
const decisionOK = `{"observation":"她愿意考虑坦白，尚未出发。","changes":[{"character_id":"mathilde","text":"我准备先向朋友坦白遗失，听听她的意见。","kind":"intent"}],"events":[],"diverged":true}`
const judgeOK = `{"ok":true,"observation_check":{"field":"observation","ok":true,"reason":"观察忠实于对话和事件"}}`
const endingOK = `{"ok":true,"field_checks":[{"field":"resolution","ok":true,"reason":"结局说明有依据"},{"field":"story","ok":true,"reason":"正文有依据"},{"field":"new_timeline","ok":true,"reason":"时间线有依据"},{"field":"contributions","ok":true,"reason":"贡献有真实回合依据"}]}`

func TestSnapshotKnowledgeAndPrivateHistory(t *testing.T) {
	e, _, o, b := setup(t)
	s, err := e.Snapshot(b, "mathilde")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(s)
	for _, secret := range []string{"五百", "仿制", "十年", "f3", "f6"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("secret in actor snapshot: %s", secret)
		}
	}
	b.Memories = append(b.Memories, Memory{CharacterID: "mathilde", Text: "私人暗号"})
	b.Turns = append(b.Turns, Turn{CharacterID: "mathilde", Message: "私人暗号", Reply: "仅她知晓"})
	husband, _ := e.Snapshot(b, "loisel")
	raw, _ = json.Marshal(husband)
	if strings.Contains(string(raw), "私人暗号") {
		t.Fatal("private context leaked")
	}
	if _, err = e.Snapshot(b, "forestier"); err == nil {
		t.Fatal("offstage character allowed")
	}
	early, _ := e.Create(o, 2)
	friend, _ := e.Snapshot(early, "forestier")
	raw, _ = json.Marshal(friend)
	if !strings.Contains(string(raw), "五百") || strings.Contains(string(raw), "找不到") {
		t.Fatal("owner knowledge time incorrect")
	}
}
func TestTurnCommitRetryForkAndRestart(t *testing.T) {
	e, c, o, b := setup(t, actorOK, decisionOK, judgeOK, actorOK, decisionOK, judgeOK)
	req := uuid.NewString()
	ctx := context.Background()
	out, err := e.Turn(ctx, o, b.ID, "mathilde", "先去坦白吧", req, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Revision != 2 || len(out.Memories) != 2 || len(out.Turns) != 1 {
		t.Fatal("missing committed effects")
	}
	if strings.Contains(c.calls[0], "五百") || strings.Contains(c.calls[0], "facts\":null") {
		t.Fatal("bad actor projection")
	}
	retry, err := e.Turn(ctx, o, b.ID, "mathilde", "先去坦白吧", req, 1, false)
	if err != nil || retry.Revision != 2 || len(c.calls) != 3 {
		t.Fatal("retry not idempotent")
	}
	if _, err = e.Turn(ctx, o, b.ID, "mathilde", "", req, 2, true); err == nil {
		t.Fatal("request kind reuse allowed")
	}
	if _, err = e.Turn(ctx, o, b.ID, "mathilde", "继续", uuid.NewString(), 1, false); !errors.Is(err, ErrConflict) {
		t.Fatal("stale version not rejected")
	}
	child, err := e.Fork(o, b.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Turn(ctx, o, b.ID, "mathilde", "继续", uuid.NewString(), 2, false); err != nil {
		t.Fatal(err)
	}
	restored, _ := New(e.Root, nil, "test", "agent")
	frozen, err := restored.Get(o, child.ID)
	if err != nil || frozen.Revision != 2 || frozen.ParentID != b.ID {
		t.Fatal("fork not frozen across restart")
	}
	if _, err = restored.Get(uuid.NewString(), child.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("visitor isolation broken")
	}
	p, _ := e.path(o, b.ID)
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal("private file permissions")
	}
}

func TestActorFactReferencesOnlyResolveWithinVisibleSnapshot(t *testing.T) {
	for _, tc := range []struct {
		id    string
		allow bool
	}{{"f2", true}, {"f3", false}, {"nonexistent", false}} {
		t.Run(tc.id, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"reply": "我愿意考虑先坦白，不仓促买替代品。", "evidence_ids": []string{tc.id}})
			e, _, owner, b := setup(t, string(raw), decisionOK, judgeOK)
			result, err := e.Turn(context.Background(), owner, b.ID, "mathilde", "先去坦白吧", uuid.NewString(), b.Revision, false)
			if tc.allow {
				if err != nil {
					t.Fatal(err)
				}
				snap, _ := e.Snapshot(b, "mathilde")
				want := ""
				for _, f := range snap.Facts {
					if f.ID == tc.id {
						want = f.EvidenceID
					}
				}
				if want == "" || len(result.Turns[0].EvidenceIDs) != 1 || result.Turns[0].EvidenceIDs[0] != want {
					t.Fatal("did not persist canonical source reference")
				}
			} else {
				if err == nil {
					t.Fatal("unseen fact accepted")
				}
				stored, _ := e.Get(owner, b.ID)
				if stored.Revision != b.Revision {
					t.Fatal("failed source check mutated story")
				}
			}
		})
	}
}
func TestRejectedGenerationDoesNotWrite(t *testing.T) {
	cases := [][]string{{`{"reply":"秘密","evidence_ids":["e7"]}`}, {actorOK, `{"observation":"串写","changes":[{"character_id":"loisel","text":"偷听了私聊","kind":"heard"}]}`}, {actorOK, decisionOK, `{"ok":false,"reason":"未完成行动"}`}, {`not json`}}
	for i, replies := range cases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			e, _, o, b := setup(t, replies...)
			_, err := e.Turn(context.Background(), o, b.ID, "mathilde", "hello", uuid.NewString(), 1, false)
			if err == nil {
				t.Fatal("should reject")
			}
			got, _ := e.Get(o, b.ID)
			if got.Revision != 1 || len(got.Memories) != 0 {
				t.Fatal("rejected changes persisted")
			}
		})
	}
}
func TestAdvanceHasSharedConsequencesWithoutFutureInheritance(t *testing.T) {
	next := `{"observation":"她前去拜访，但尚未开口。","changes":[{"character_id":"mathilde","text":"我已到朋友家中。","kind":"experience"},{"character_id":"forestier","text":"旧友到家里来找我。","kind":"experience"}],"events":["旧友会面"],"diverged":true,"next_scene":{"title":"门前","place":"朋友家","summary":"佛来思节夫人开了门，玛蒂尔德站在门口。","characters":["mathilde","forestier"]}}`
	e, _, o, b := setup(t, next, judgeOK)
	out, err := e.Turn(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), 1, true)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := e.Snapshot(out, "forestier")
	raw, _ := json.Marshal(s)
	if !strings.Contains(string(raw), "旧友到家") || strings.Contains(string(raw), "十年") || strings.Contains(string(raw), "找不到") {
		t.Fatal("advance leaked facts")
	}
	if out.Anchor != 4 || out.Scene.Title != "门前" {
		t.Fatal("anchor incorrectly advanced into original future")
	}
}
func TestHTTPIsolationAndOrigin(t *testing.T) {
	e, _, _, _ := setup(t)
	mux := http.NewServeMux()
	e.registerCatalog(mux)
	h := Middleware(mux)
	run := func(method, path, body, origin string, c *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if c != nil {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := run("POST", "/api/story/branches", `{"scene":4}`, "", nil)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var b Branch
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	cookie := w.Result().Cookies()[0]
	if run("GET", "/api/story/branches/"+b.ID, "", "", cookie).Code != 200 {
		t.Fatal("cannot resume")
	}
	if run("GET", "/api/story/branches/"+b.ID, "", "", nil).Code != 404 {
		t.Fatal("visitor crossover")
	}
	if run("POST", "/api/story/branches", `{"scene":4}`, "https://evil.example", cookie).Code != 403 {
		t.Fatal("cross origin allowed")
	}
}
func TestReadingWholeSourceCheckpointsAndResume(t *testing.T) {
	e, c, o, _ := setup(t, `{"scene":1,"observation":"衣服背后的处境","question":"为何难过？","evidence_ids":["e1"]}`, `invalid`)
	wait := func(status string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if e.Reading(o).Status == status {
				e.mu.Lock()
				running := e.running[o]
				e.mu.Unlock()
				if !running {
					return
				}
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("not %s: %+v", status, e.Reading(o))
	}
	if err := e.StartReading(o); err != nil {
		t.Fatal(err)
	}
	wait("failed")
	if e.Reading(o).Completed != 1 {
		t.Fatal("checkpoint lost")
	}
	if !strings.Contains(c.calls[0], "my necklace was paste") {
		t.Fatal("not reading actual full source")
	}
	// Resume from scene 2, never replay scene 1.
	for i := 2; i <= 7; i++ {
		raw, _ := json.Marshal(Insight{Scene: i, Observation: "解读", Question: "问题", EvidenceIDs: []string{e.Book.Scenes[i-1].EvidenceIDs[0]}})
		c.replies = append(c.replies, string(raw))
	}
	if err := e.StartReading(o); err != nil {
		t.Fatal(err)
	}
	wait("completed")
	r := e.Reading(o)
	if r.Completed != 7 || len(r.Insights) != 7 || len(r.SourceHash) != 64 {
		t.Fatal("incomplete reading")
	}
	restored, _ := New(e.Root, nil, "test", "observe")
	if restored.Reading(o).Completed != 7 {
		t.Fatal("reading not persisted")
	}
}
func TestModes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "invalid"} {
		e, _, o, b := setup(t)
		e.Mode = mode
		if _, err := e.Turn(context.Background(), o, b.ID, "mathilde", "test", uuid.NewString(), 1, false); err == nil {
			t.Fatal("mode allowed writing")
		}
	}
}
