package evals

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRoleSuiteInventory(t *testing.T) {
	suite, err := LoadRoleSuite(filepath.Join("..", "..", "evals", "t4", "role_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Cases) != 36 {
		t.Fatalf("cases=%d", len(suite.Cases))
	}
	counts := map[string]int{}
	for _, c := range suite.Cases {
		counts[c.Category]++
	}
	for category, count := range counts {
		if count != 6 {
			t.Fatalf("%s cases=%d", category, count)
		}
	}
}

func TestRoleRulesEnforceIsolationAndStructure(t *testing.T) {
	c := RoleCase{Expect: RoleExpect{
		AllowedDirectorActions: []string{"no_change"},
		MustNotLeakBackstage:   true, PrivateSandbox: true,
	}}
	result := EvaluateRoleRules(c, RoleObservation{
		DirectorAction: "no_change", StructuralPassed: true, PrivateContained: true,
	})
	if !result.Passed {
		t.Fatalf("expected pass: %#v", result)
	}
	result = EvaluateRoleRules(c, RoleObservation{
		DirectorAction: "no_change", StructuralPassed: true,
		PrivateContained: true, BackstageLeaked: true,
	})
	if result.Passed {
		t.Fatal("backstage leak passed")
	}
}

type roleJudge struct{ out string }

func (r roleJudge) Complete(context.Context, string, string) (string, error) { return r.out, nil }

func TestJudgeRoleSemantics(t *testing.T) {
	result, err := JudgeRoleSemantics(context.Background(), roleJudge{out: "{\"passed\":true,\"reason\":\"角色承认时间边界\"}"}, RoleCase{
		ID: "ST2", Role: RoleFixture{DisplayName: "人物"}, UserText: "未来如何",
		Expect: RoleExpect{SemanticRules: []string{"承认不知道"}},
	}, RoleObservation{Answer: "那在我的时代之后，我无法知道。", DirectorAction: "no_change"})
	if err != nil || !result.Passed {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
