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
