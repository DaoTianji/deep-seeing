package story

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"reflect"
	"strings"
	"testing"
)

func TestEndingRevisionCannotInventEventsToSupportItsOwnProse(t *testing.T) {
	e, _, o, b := setup(t)
	b = seedVisitor(t, e, b)
	var d Decision
	_ = json.Unmarshal([]byte(closingDecision(t, b.Turns[0].ID)), &d)
	d.Events = []string{"他们坦白了遗失经过，尚未决定赔偿方式。"}
	bad := *d.Ending
	bad.Story += "他们当天已经支付五百法郎。"
	d.Ending = &bad
	candidate, _ := json.Marshal(d)
	corrected := endingFixture(b.Turns[0].ID)
	corrected.Story = "他们在朋友面前坦白遗失，曾经不敢说出口的话终于说了出来。赔偿尚未商定，项链也仍然没有找回，但不再隐瞒的选择已经让故事走向不同的结尾。他们愿意承担责任，也愿意接受结果仍然未知。"
	revised, _ := json.Marshal(map[string]any{"title": corrected.Title, "story": corrected.Story, "resolution": corrected.Resolution, "new_timeline": corrected.NewTimeline, "contributions": corrected.Contributions, "events": []string{"注入的付款事件"}})
	c := &promptCapture{scripted: scripted{replies: []string{string(candidate), judgeOK, `{"ok":false,"reason":"事件只到坦白，正文额外写了已付款。"} `, string(revised), endingOK}}}
	e.Chat = c
	out, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), b.Revision, false, true, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 5 || out.Revision != 3 || !reflect.DeepEqual(out.Events, d.Events) {
		t.Fatal("revision changed approved world or call bound")
	}
	if strings.Contains(out.Ending.Story, "五百") || out.Ending.Story != corrected.Story {
		t.Fatal("rejected prose survived")
	}
	if !strings.HasPrefix(c.systems[2], endingEvidencePrompt) || !strings.HasPrefix(c.systems[3], endingRevisionPrompt) || !strings.Contains(c.calls[3], "正文额外写了已付款") {
		t.Fatal("feedback did not guide revision")
	}
	if strings.Contains(c.calls[2], "authority\":{\"ending\"") {
		t.Fatal("candidate treated as authority")
	}
}

func TestEndingRevisionIsBoundedAndNeverCommitsAnotherFailure(t *testing.T) {
	e, c, o, b := setup(t)
	b = seedVisitor(t, e, b)
	var d Decision
	_ = json.Unmarshal([]byte(closingDecision(t, b.Turns[0].ID)), &d)
	revised, _ := json.Marshal(d.Ending)
	c.replies = []string{closingDecision(t, b.Turns[0].ID), judgeOK, `{"ok":false,"reason":"存在额外事实。"} `, string(revised), `{"ok":false,"reason":"仍然没有依据。"} `}
	_, err := e.TurnWithOptions(context.Background(), o, b.ID, "mathilde", "", uuid.NewString(), b.Revision, false, true, 10)
	if err == nil || len(c.calls) != 5 {
		t.Fatal("unbounded or accepted failed revision")
	}
	got, _ := e.Get(o, b.ID)
	if got.Revision != b.Revision || got.Ending != nil || len(got.Events) != len(b.Events) {
		t.Fatal("partial world change committed")
	}
}
