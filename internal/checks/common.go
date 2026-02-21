package checks

import (
	"context"
	"netcheck/internal/execx"
	"strings"
	"time"
)

func runWithTimeout(parent context.Context, timeoutSec int, ex execx.Executor, name string, args ...string) execx.Result {
	t := time.Duration(timeoutSec) * time.Second
	if t <= 0 {
		t = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, t)
	defer cancel()
	return ex.Run(ctx, name, args...)
}

func isInterruptedError(err error) bool {
	if err == nil {
		return false
	}
	if execx.HasTimeoutError(err) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "context canceled") ||
		strings.Contains(s, "signal: killed") ||
		strings.Contains(s, "killed")
}
