package execx

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRealExecutorRun(t *testing.T) {
	r := RealExecutor{}.Run(context.Background(), "sh", "-c", "echo hi")
	if r.Err != nil || r.ExitCode != 0 {
		t.Fatalf("unexpected error: %v code=%d", r.Err, r.ExitCode)
	}
}

func TestRealExecutorLookPath(t *testing.T) {
	if _, err := (RealExecutor{}).LookPath("sh"); err != nil {
		t.Fatal(err)
	}
}

func TestFakeExecutorContextTimeout(t *testing.T) {
	f := &FakeExecutor{Delays: map[string]time.Duration{"ping -c 1 1.1.1.1": 2 * time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	r := f.Run(ctx, "ping", "-c", "1", "1.1.1.1")
	if !errors.Is(r.Err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", r.Err)
	}
}

func TestHasTimeoutError(t *testing.T) {
	if !HasTimeoutError(context.DeadlineExceeded) {
		t.Fatal("expected true")
	}
}
