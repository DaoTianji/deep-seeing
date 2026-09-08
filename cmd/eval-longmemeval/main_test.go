package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInferenceBoundaryDropsGoldAndLabels(t *testing.T) {
	var x Item
	err := json.Unmarshal([]byte(`{"question_id":"gold_abs","question_type":"knowledge-update","question":"What was the code?","question_date":"2023/05/30 (Tue) 23:40","answer":"SECRET_GOLD","answer_session_ids":["GOLD_SOURCE"],"haystack_dates":["2023/05/20 (Sat) 02:21"],"haystack_session_ids":["GOLD_SOURCE"],"haystack_sessions":[[{"role":"user","content":"The code was violet.","has_answer":true}]]}`), &x)
	if err != nil {
		t.Fatal(err)
	}
	if err = validate(x); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(x.input())
	for _, secret := range []string{"SECRET_GOLD", "GOLD_SOURCE", "gold_abs", "knowledge-update", "has_answer"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("inference leaked %s", secret)
		}
	}
	if !strings.Contains(string(raw), "violet") {
		t.Fatal("legitimate content was removed")
	}
}

func TestHistoricalTimestampAndSpeakerArePreserved(t *testing.T) {
	s := sessionText("2023/05/20 (Sat) 02:21", []Turn{{Role: "user", Content: "Earlier value"}, {Role: "assistant", Content: "Correction"}})
	for _, v := range []string{"2023/05/20 (Sat) 02:21", "user: Earlier value", "assistant: Correction"} {
		if !strings.Contains(s, v) {
			t.Fatal("missing", v)
		}
	}
	x := Item{ID: "id", Question: "q", Date: "2023/05/30 (Tue) 23:40", Sessions: [][]Turn{{{Role: "user", Content: "text"}}}, Dates: []string{"bad-date"}, SessionIDs: []string{"a"}}
	if validate(x) == nil {
		t.Fatal("invalid historical date accepted")
	}
}

func TestReferenceMappingIsPostInferenceAndDeduplicated(t *testing.T) {
	x := Item{SessionIDs: []string{"source-a", "source-b"}}
	r := Result{SessionMap: map[string]int{"ep-one": 1}}
	got := sessionIDs([]string{"missing", "ep-one", "ep-one"}, r, x)
	if len(got) != 1 || got[0] != "source-b" {
		t.Fatalf("bad mapping %v", got)
	}
}

func TestOnlyInfrastructureFailuresAreRetried(t *testing.T) {
	if retryable(nil) {
		t.Fatal("success retry")
	}
	// Semantic wrong answers are successful calls, not retry candidates.
	if retryable(&testError{"maximum steps exceeded"}) {
		t.Fatal("agent behavior must remain scored")
	}
	if !retryable(&testError{"connection reset by peer"}) {
		t.Fatal("transport retry missing")
	}
}

type testError struct{ s string }

func (e *testError) Error() string { return e.s }
