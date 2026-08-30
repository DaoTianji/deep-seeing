package theater

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"deep-seeing/internal/identity"
)

func testScope() identity.TenantScope { return identity.TenantScope{UserID: "tester", AgentID: "an"} }

func TestModeParsing(t *testing.T) {
	for raw, want := range map[string]Mode{"": ModeOff, "off": ModeOff, "observe": ModeObserve, "AGENT": ModeAgent, "invalid": ModeOff} {
		if got := ParseMode(raw); got != want {
			t.Fatalf("ParseMode(%q)=%q want %q", raw, got, want)
		}
	}
}

func TestRoleLifecycleAndRecovery(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "林舟", Kind: RoleCharacter, SubjectClass: SubjectFictional, Identity: "虚构航海者"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := store.Enter(ctx, testScope(), d.ID); err == nil {
		t.Fatal("draft role must not enter")
	}
	d, err = store.SetValidation(ctx, d.ID, ValidationReport{Passed: true})
	if err != nil {
		t.Fatal(err)
	}
	d, err = store.Publish(ctx, d.ID)
	if err != nil || d.Status != DefinitionReady {
		t.Fatalf("publish: %#v %v", d, err)
	}
	_, inst, session, err := store.Enter(ctx, testScope(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != SessionActive || inst.MainWorldlineID == "" {
		t.Fatalf("unexpected enter: %#v %#v", inst, session)
	}
	if err := store.AppendTranscript(ctx, session.ID, TranscriptMessage{Channel: ChannelStage, Role: "user", Content: "你好"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTranscript(ctx, session.ID, TranscriptMessage{Channel: ChannelBackstage, Role: "user", Content: "安，观察他"}); err != nil {
		t.Fatal(err)
	}
	stage, _ := store.ReadTranscript(ctx, session.ID, ChannelStage, 10)
	backstage, _ := store.ReadTranscript(ctx, session.ID, ChannelBackstage, 10)
	if len(stage) != 1 || len(backstage) != 1 || stage[0].Content == backstage[0].Content {
		t.Fatal("transcripts not isolated")
	}
	recovered, changed, err := store.Recover(ctx)
	if err != nil || !changed || recovered.Status != SessionPaused {
		t.Fatalf("recover: %#v %v %v", recovered, changed, err)
	}
	if _, err := store.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	closed, err := store.Exit(ctx, "user_command", false)
	if err != nil || closed.Status != SessionCompleted {
		t.Fatalf("exit: %#v %v", closed, err)
	}
}

func TestCanonicalForkKeepsParent(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(t.TempDir())
	d, _ := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "旅人"})
	d, _ = store.SetValidation(ctx, d.ID, ValidationReport{Passed: true})
	d, _ = store.Publish(ctx, d.ID)
	_, inst, _, _ := store.Enter(ctx, testScope(), d.ID)
	parentID := inst.CurrentWorldlineID
	branch, next, err := store.ForkWorldline(ctx, "ract_1", "另一条人生")
	if err != nil {
		t.Fatal(err)
	}
	if branch.ParentWorldlineID != parentID || next.CurrentWorldlineID != branch.ID || next.MainWorldlineID != parentID {
		t.Fatalf("bad fork: %#v %#v", branch, next)
	}
	if _, err := store.GetWorldline(ctx, parentID); err != nil {
		t.Fatal("parent must remain", err)
	}
}

func TestPrivateRolePermissions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, _ := NewStore(root)
	d, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{
		DisplayName: "私人角色", SubjectClass: SubjectLivingPrivate,
		ToolPolicy: RoleToolPolicy{Allowed: []string{"search_web"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !d.PrivateSandbox || len(d.ToolPolicy.Allowed) != 0 {
		t.Fatalf("private role escaped sandbox: %#v", d.ToolPolicy)
	}
	src, err := store.AddSource(ctx, d.ID, "私人笔记", "upload", "", "text/plain", []byte("private"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(src.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("material mode=%o", info.Mode().Perm())
	}
}

func TestPublishRequiresValidation(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	d, _ := store.CreateDefinition(context.Background(), testScope(), RoleDefinitionWrite{DisplayName: "未验收"})
	if _, err := store.Publish(context.Background(), d.ID); err == nil {
		t.Fatal("publish without validation must fail")
	}
}
