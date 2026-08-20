package graph_test

import (
	"strings"
	"testing"

	"deep-seeing/internal/graph"
)

func TestFormatCompactRecallPlaceholder(t *testing.T) {
	text, tr := graph.FormatCompactRecall(graph.Bond{}, "")
	if text != graph.BondPlaceholder {
		t.Fatalf("got %q", text)
	}
	if !tr.Placeholder || !tr.StrategyOmit {
		t.Fatalf("trace=%+v", tr)
	}
}

func TestFormatCompactRecallOmitsStrategySoT(t *testing.T) {
	b := graph.Bond{
		PersonID:   "user:mudnet",
		Boundaries: "不要居高临下安慰",
		Strategy:   "应该直接给方案", // must not appear as SoT
		Style:      "喜欢先看框架\n不喜欢长篇铺垫\n第三条\n第四条\n第五条\n第六条应被裁掉",
		Concerns:   "长期关注 AI Agent",
	}
	text, tr := graph.FormatCompactRecall(b, "今天天气")
	if strings.Contains(text, "应该直接给方案") {
		t.Fatalf("strategy leaked: %s", text)
	}
	if !strings.Contains(text, "不要居高临下安慰") {
		t.Fatalf("boundaries missing: %s", text)
	}
	if !strings.Contains(text, "喜欢先看框架") {
		t.Fatalf("interaction missing: %s", text)
	}
	if strings.Contains(text, "第六条应被裁掉") {
		t.Fatalf("top-N failed: %s", text)
	}
	if strings.Contains(text, "长期关注") {
		t.Fatalf("unrelated priorities should stay out: %s", text)
	}
	if tr.StrategyOmit {
		// ok
	}
	found := false
	for _, s := range tr.Slots {
		if s == graph.SlotBoundaries {
			found = true
		}
	}
	if !found {
		t.Fatalf("slots=%v", tr.Slots)
	}
}

func TestFormatCompactRecallConditionalPriorities(t *testing.T) {
	b := graph.Bond{
		PersonID: "user:mudnet",
		Basics:   "称呼偏好: 安",
		Concerns: "长期关注 AI Agent 与职业成长",
	}
	text, tr := graph.FormatCompactRecall(b, "我们聊聊 agent 架构")
	if !strings.Contains(text, "AI Agent") {
		t.Fatalf("expected conditional hit: %s", text)
	}
	hasPri := false
	for _, s := range tr.Slots {
		if s == graph.SlotPriorities {
			hasPri = true
		}
	}
	if !hasPri {
		t.Fatalf("slots=%v", tr.Slots)
	}
}

func TestFormatCompactRecallStrategyCache(t *testing.T) {
	b := graph.Bond{
		PersonID:         "user:mudnet",
		Boundaries:       "不要居高临下",
		Version:          3,
		StrategyCache:    "先给框架，再问约束；避免长篇铺垫",
		StrategyCacheVer: 3,
	}
	text, tr := graph.FormatCompactRecall(b, "")
	if tr.StrategyOmit {
		t.Fatalf("expected strategy inject: %+v", tr)
	}
	if !strings.Contains(text, "先给框架") {
		t.Fatalf("missing cache: %s", text)
	}
	b.Version = 4 // SoT bumped → stale cache
	text2, tr2 := graph.FormatCompactRecall(b, "")
	if !tr2.StrategyOmit {
		t.Fatalf("stale cache should omit")
	}
	if strings.Contains(text2, "先给框架") {
		t.Fatalf("stale leaked: %s", text2)
	}
}
