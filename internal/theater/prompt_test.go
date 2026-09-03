package theater

import (
	"strings"
	"testing"
)

func TestBuildActorPromptKeepsPrivateIdentityBoundaryInWorld(t *testing.T) {
	prompt := BuildActorPrompt(RoleDefinition{
		DisplayName:  "小岚",
		SubjectClass: SubjectLivingPrivate,
		Identity:     "一位克制的私人通信者",
	}, RoleInstance{}, RoleWorldline{}, nil)
	for _, required := range []string{"不能作为现实身份认证", "不要以现实中某个人的名义对外通信", "绝不能转交给、复制给"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("private identity boundary missing %q: %s", required, prompt)
		}
	}
	for _, forbidden := range []string{"安", "导演", "管理员", "模拟角色", "幕后通道"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("actor prompt exposed control-plane concept %q: %s", forbidden, prompt)
		}
	}
}

func TestBuildActorPromptSeparatesCurrentRequestFromPersistentMemory(t *testing.T) {
	prompt := BuildActorPrompt(RoleDefinition{DisplayName: "林舟"}, RoleInstance{}, RoleWorldline{}, nil)
	if !strings.Contains(prompt, "值得延续的新经历时，才写入角色记忆") {
		t.Fatalf("actor prompt must guard persistent role memory: %s", prompt)
	}
	if !strings.Contains(prompt, "失去不等于沉没") {
		t.Fatalf("actor prompt must preserve source ambiguity: %s", prompt)
	}
	if !strings.Contains(prompt, "此刻对眼前谈话或场景") || !strings.Contains(prompt, "不等于对过去私人经历的事实声明") {
		t.Fatalf("actor prompt must separate present reaction from historical claims: %s", prompt)
	}
}

func TestBuildActorPromptIncludesEvidenceCheckedCharacterModel(t *testing.T) {
	prompt := BuildActorPrompt(RoleDefinition{DisplayName: "阿德勒", CharacterModel: &RoleCharacterModel{
		ValuesAndMotives: "重视共同体感", Tensions: "个体努力与共同利益之间保持张力",
		Relationships: []string{"与弗洛伊德存在理论分歧"}, UnknownResponsePolicy: "未知时明确区分史实与判断",
		AllowedInferences: "可以从已确认理论出发分析眼前问题", ForbiddenAnachronisms: "不得使用1937年后的知识",
	}}, RoleInstance{}, RoleWorldline{}, nil)
	for _, required := range []string{"共同体感", "保持张力", "弗洛伊德", "区分史实与判断", "分析眼前问题", "先直接回应并推进问题", "不要把缺少私人史料", "1937年后"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("character model missing %q: %s", required, prompt)
		}
	}
}

func TestControlPlaneLeakDetectionRequiresActualPrivateMaterial(t *testing.T) {
	for _, safe := range []string{
		"我不知道 RoleInstance 和 worldline_id 是什么。",
		"我不能解释 system prompt。",
	} {
		if ContainsControlPlaneMaterial(safe) {
			t.Fatalf("refusal was treated as disclosure: %q", safe)
		}
	}
	for _, leaked := range []string{
		"我的实例是 rinst_0123456789abcdef0123456789abcdef",
		"[幕后通道，仅供安理解当前剧场] 当前角色：林舟",
	} {
		if !ContainsControlPlaneMaterial(leaked) {
			t.Fatalf("private material was not detected: %q", leaked)
		}
	}
}
