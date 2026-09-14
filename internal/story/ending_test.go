package story

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func endingFixture(turnID string) *Ending {
	return &Ending{Title: "不必偿还的十年", Story: "他们终于在朋友面前说出了项链遗失的真相。原本准备独自承担的恐惧，变成了一场诚实的交谈。项链仍未找回，但他们不再仓促借债购买昂贵替代品，愿意承认疏忽，一起商量下一步。", Resolution: "秘密已经说开，昂贵赔偿不再是唯一选项；失物可以继续寻找，不必把每一步都写完。", NewTimeline: []string{"项链遗失，他们惊慌失措。", "来访者劝说坦白，他们开始考虑。", "双方把话说开，决定不仓促借债。"}, Contributions: []Contribution{{TurnID: turnID, Text: "你鼓励坦白，帮助他们考虑隐瞒之外的选择。"}}}
}
func closingDecision(t *testing.T, id string) string {
	t.Helper()
	d := Decision{Observation: "核心矛盾收束，保留项链仍未找回的事实。", NextScene: &Scene{Title: "坦白之后", Place: "朋友家中", Summary: "他们停止仓促赔偿的打算，坐下来认真商量。", Characters: []string{"mathilde", "forestier"}}, Ending: endingFixture(id)}
	raw, _ := json.Marshal(d)
	return string(raw)
}
func seedVisitor(t *testing.T, e *Engine, b Branch) Branch {
	t.Helper()
	b.Turns = []Turn{{ID: uuid.NewString(), Kind: "chat", CharacterID: "mathilde", Message: "不妨先去坦白", Reply: "我会认真考虑", Revision: 2}}
	b.Revision = 2
	if err := e.save(b); err != nil {
		t.Fatal(err)
	}
	return b
}
func TestAutomaticEndingIsAtomicImmutableAndForkable(t *testing.T) {
	e, c, o, b := setup(t)
	b = seedVisitor(t, e, b)
	c.replies = []string{closingDecision(t, b.Turns[0].ID), judgeOK, endingOK}
	req := uuid.NewString()
	out, err := e.Turn(context.Background(), o, b.ID, "mathilde", "", req, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if out.Ending == nil || out.Ending.Revision != 3 || out.Ending.Created.IsZero() {
		t.Fatal("ending not committed")
	}
	again, err := e.Turn(context.Background(), o, b.ID, "mathilde", "", req, 2, true)
	if err != nil || again.Revision != 3 {
		t.Fatal("ending retry not idempotent")
	}
	if _, err = e.Turn(context.Background(), o, b.ID, "mathilde", "再继续", uuid.NewString(), 3, false); err == nil {
		t.Fatal("completed branch writable")
	}
	fresh, _ := New(e.Root, c, "test", "agent")
	restored, _ := fresh.Get(o, b.ID)
	if restored.Ending == nil {
		t.Fatal("ending lost on restart")
	}
	child, err := fresh.Fork(o, b.ID, 3)
	if err != nil || child.Ending != nil {
		t.Fatal("cannot explicitly reopen through fork")
	}
	parent, _ := fresh.Get(o, b.ID)
	if parent.Ending == nil {
		t.Fatal("fork altered completed parent")
	}
}
func TestManualEndingRequiredAndCanFinishAtTurnLimit(t *testing.T) {
	e, c, o, b := setup(t)
	b = seedVisitor(t, e, b)
	for len(b.Turns) < 40 {
		b.Turns = append(b.Turns, Turn{ID: uuid.NewString(), Kind: "advance"})
	}
	_ = e.save(b)
	c.replies = []string{closingDecision(t, b.Turns[0].ID), judgeOK, endingOK}
	out, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), 2, false, true, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Ending == nil || out.Influence != 10 || out.Turns[40].Kind != "finish" {
		t.Fatal("manual end did not close")
	}
}
func TestInvalidEndingNeverCommits(t *testing.T) {
	for _, kind := range []string{"unknown_contribution", "short_story", "judge_reject", "finish_missing"} {
		t.Run(kind, func(t *testing.T) {
			e, c, o, b := setup(t)
			b = seedVisitor(t, e, b)
			var d Decision
			_ = json.Unmarshal([]byte(closingDecision(t, b.Turns[0].ID)), &d)
			switch kind {
			case "unknown_contribution":
				d.Ending.Contributions[0].TurnID = uuid.NewString()
			case "short_story":
				d.Ending.Story = "假的摘要"
			case "finish_missing":
				d.Ending = nil
			}
			raw, _ := json.Marshal(d)
			judge := judgeOK
			if kind == "judge_reject" {
				judge = `{"ok":false}`
			}
			c.replies = []string{string(raw), judge}
			_, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), 2, false, true, 5)
			if err == nil {
				t.Fatal("invalid ending accepted")
			}
			got, _ := e.Get(o, b.ID)
			if got.Ending != nil || got.Revision != 2 {
				t.Fatal("partial ending written")
			}
		})
	}
}

type promptCapture struct {
	scripted
	systems []string
}

func (p *promptCapture) Complete(ctx context.Context, system, input string) (string, error) {
	p.systems = append(p.systems, system)
	return p.scripted.Complete(ctx, system, input)
}
func TestInfluenceActuallyReachesActorAndDirector(t *testing.T) {
	for _, level := range []int{5, 10} {
		e, _, o, b := setup(t)
		c := &promptCapture{scripted: scripted{replies: []string{actorOK, decisionOK, judgeOK}}}
		e.Chat = c
		out, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "请考虑坦白", uuid.NewString(), 1, false, false, level)
		if err != nil {
			t.Fatal(err)
		}
		if out.Influence != level || !strings.Contains(c.systems[0], influencePrompt(level)) || !strings.Contains(c.systems[1], influencePrompt(level)) {
			t.Fatal("slider was cosmetic")
		}
		if !strings.Contains(c.systems[0], "不要每轮都反问") {
			t.Fatal("question loop prompt missing")
		}
	}
}

func TestProgressionUsesWorldTransactionJudgeNotPrivateChatJudge(t *testing.T) {
	for _, finish := range []bool{false, true} {
		e, _, o, b := setup(t)
		b = seedVisitor(t, e, b)
		c := &promptCapture{scripted: scripted{replies: []string{closingDecision(t, b.Turns[0].ID), judgeOK, endingOK}}}
		e.Chat = c
		if _, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), b.Revision, !finish, finish, 10); err != nil {
			t.Fatal(err)
		}
		if len(c.systems) != 3 || !strings.HasPrefix(c.systems[1], advanceJudgePrompt) || strings.Contains(c.systems[1], "若turn_kind为chat") {
			t.Fatal("world progression judged as private chat")
		}
		if !strings.Contains(c.systems[1], "明确的在场、听闻或观察渠道") {
			t.Fatal("progression lost knowledge propagation check")
		}
		if !strings.Contains(c.systems[0], memoryStateContract) || !strings.Contains(c.systems[1], memoryStateContract) {
			t.Fatal("writer and reviewer disagree about memory kind semantics")
		}
	}
	e, _, o, b := setup(t)
	c := &promptCapture{scripted: scripted{replies: []string{actorOK, decisionOK, judgeOK}}}
	e.Chat = c
	if _, err := e.Turn(context.Background(), o, b.ID, "mathilde", "先想一想", uuid.NewString(), b.Revision, false); err != nil {
		t.Fatal(err)
	}
	if len(c.systems) != 3 || !strings.HasPrefix(c.systems[2], judgePrompt) {
		t.Fatal("private chat lost its non-advancement judge")
	}
}

func TestBranchWriterReceivesPastWorldNotOriginalFuture(t *testing.T) {
	e, _, o, b := setup(t)
	b = seedVisitor(t, e, b)
	c := &promptCapture{scripted: scripted{replies: []string{closingDecision(t, b.Turns[0].ID), judgeOK, endingOK}}}
	e.Chat = c
	if _, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), b.Revision, false, true, 10); err != nil {
		t.Fatal(err)
	}
	var input struct {
		Book struct {
			Scenes []Scene `json:"scenes"`
			Facts  []Fact  `json:"facts"`
			Notes  []struct {
				Scene int    `json:"scene"`
				Note  string `json:"note"`
			} `json:"editorial_notes"`
		} `json:"book"`
		Branch Branch `json:"branch"`
	}
	if err := json.Unmarshal([]byte(c.calls[0]), &input); err != nil {
		t.Fatal(err)
	}
	for _, scene := range input.Book.Scenes {
		if scene.ID > b.Anchor || scene.Question != "" {
			t.Fatal("future scene or editorial spoiler sent to writer")
		}
	}
	secret := false
	for _, f := range input.Book.Facts {
		if f.Since > b.Anchor {
			t.Fatal("future fact sent as world state")
		}
		if f.ID == "f3" {
			secret = true
		}
	}
	if !secret {
		t.Fatal("withheld existing private world fact from director")
	}
	if len(input.Book.Notes) == 0 {
		t.Fatal("source uncertainty missing from writer")
	}
	for _, note := range input.Book.Notes {
		if note.Scene > b.Anchor {
			t.Fatal("future editorial note reached writer")
		}
	}
	if !strings.Contains(c.calls[0], "没有确定遗失的精确地点") || !strings.Contains(c.calls[2], "没有确定遗失的精确地点") {
		t.Fatal("writer or final prose audit lost known source uncertainty")
	}
	if input.Branch.Scene.Question != "" {
		t.Fatal("branch editorial question revealed future")
	}
	if !strings.Contains(c.calls[1], "我们辛苦工作、节省了十年") {
		t.Fatal("independent judge lost full source comparison")
	}
	if e.Book.Scenes[3].Question == "" {
		t.Fatal("source map mutated")
	}
}
func TestInfluenceRangeAndOldSaveDefault(t *testing.T) {
	e, _, o, b := setup(t)
	b.Influence = 0
	_ = e.save(b)
	loaded, _ := e.Get(o, b.ID)
	if loaded.Influence != 5 || loaded.Ending != nil {
		t.Fatal("legacy save incompatible")
	}
	for _, level := range []int{4, 11, -1} {
		if _, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "test", uuid.NewString(), 1, false, false, level); err == nil {
			t.Fatal("invalid influence")
		}
	}
}
