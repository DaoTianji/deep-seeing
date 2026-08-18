package runtime_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"deep-seeing/internal/runtime"
)

func TestRunCognitiveSerializes(t *testing.T) {
	q := runtime.NewExecutionQueue("agent-a")
	var concurrent int32
	var max int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.RunCognitive(context.Background(), "chat", func(context.Context) error {
				cur := atomic.AddInt32(&concurrent, 1)
				for {
					old := atomic.LoadInt32(&max)
					if cur <= old || atomic.CompareAndSwapInt32(&max, old, cur) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt32(&concurrent, -1)
				return nil
			})
		}()
	}
	wg.Wait()
	if max != 1 {
		t.Fatalf("expected serialized cognitive turns, max concurrent=%d", max)
	}
}

func TestAutoSessionID(t *testing.T) {
	got := runtime.AutoSessionID("intent-1", 2)
	if got != "auto:intent-1:2" {
		t.Fatalf("got %q", got)
	}
	if !runtime.IsAutoSession(got) {
		t.Fatal("expected auto session")
	}
	if runtime.IsAutoSession("room") {
		t.Fatal("room should not be auto")
	}
}
