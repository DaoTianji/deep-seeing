package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/transcript"
)

func TestRedisSTMAppendGetTTL(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	scope := identity.LocalCLI()
	stm, err := memory.NewRedisSTM(context.Background(), memory.RedisSTMConfig{
		Addr:        mr.Addr(),
		MaxMessages: 10,
		TTL:         time.Minute,
		Scope:       scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.Append("cli", transcript.User("hi"), transcript.Assistant("hello")); err != nil {
		t.Fatal(err)
	}
	got, err := stm.Get("cli")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Content != "hi" {
		t.Fatalf("got %+v", got)
	}
	if mr.TTL(stmKey(scope, "cli")) <= 0 {
		t.Fatal("expected positive TTL")
	}

	if err := stm.Replace("cli", []transcript.Message{transcript.Summary("旧对话摘要"), transcript.User("new")}); err != nil {
		t.Fatal(err)
	}
	got, err = stm.Get("cli")
	if err != nil || len(got) != 2 {
		t.Fatalf("replace: %v %+v", err, got)
	}
	if got[0].Role != transcript.RoleSummary {
		t.Fatalf("want summary role, got %s", got[0].Role)
	}
}

func stmKey(scope identity.TenantScope, session string) string {
	return "ds:stm:" + scope.UserID + ":" + scope.AgentID + ":" + session
}
