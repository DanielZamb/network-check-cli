package checks

import (
	"context"
	"fmt"
	"netcheck/internal/config"
	"netcheck/internal/eval"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"time"
)

type ReachabilityCheck struct{ Target string }

func (c ReachabilityCheck) ID() string    { return "reachability." + c.Target }
func (c ReachabilityCheck) Group() string { return "reachability" }

func (c ReachabilityCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if _, err := ex.LookPath("ping"); err != nil {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusSkip, Error: "ping not found"}
	}
	res := runWithTimeout(ctx, timeoutSec, ex, "ping", "-c", "10", c.Target)
	if isInterruptedError(res.Err) {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusFail, Error: res.Err.Error(), Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	if res.Err != nil && res.Stdout == "" {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusFail, Error: res.Err.Error(), DurationMS: time.Since(start).Milliseconds()}
	}
	loss, avg, jitter, p95 := parsePing(res.Stdout)
	status := eval.LowerIsBetter(loss, cfg.Thresholds.LossPassMax, cfg.Thresholds.LossWarnMax)
	if status == model.StatusPass {
		status = eval.LowerIsBetter(p95, cfg.Thresholds.RTTP95PassMaxMs, cfg.Thresholds.RTTP95WarnMaxMs)
	}
	if status == model.StatusPass {
		status = eval.LowerIsBetter(jitter, cfg.Thresholds.JitterPassMaxMs, cfg.Thresholds.JitterWarnMaxMs)
	}
	return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: status, Metrics: map[string]any{"loss_pct": loss, "avg_ms": avg, "rtt_p95_ms": p95, "jitter_ms": jitter}, Raw: res.Stdout, Error: stderrMsg(res.Stderr), DurationMS: time.Since(start).Milliseconds()}
}

func stderrMsg(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("stderr: %s", s)
}
