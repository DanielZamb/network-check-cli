package checks

import (
	"context"
	"netcheck/internal/config"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"time"
)

type PathCheck struct{ Target string }

func (c PathCheck) ID() string    { return "path." + c.Target }
func (c PathCheck) Group() string { return "path" }

func (c PathCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if _, err := ex.LookPath("mtr"); err != nil {
		return c.runTraceroute(ctx, ex, timeoutSec, start)
	}
	res := runWithTimeout(ctx, timeoutSec, ex, "mtr", "-rwzc", "10", c.Target)
	if isInterruptedError(res.Err) {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusFail, Error: res.Err.Error(), Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	if res.Err != nil {
		// mtr can be installed but unusable due to socket/capability restrictions.
		// Fall back to traceroute before failing hard.
		return c.runTraceroute(ctx, ex, timeoutSec, start)
	}
	hops, nearDestLoss := parseMTRSummary(res.Stdout)
	status := model.StatusPass
	if nearDestLoss >= 2 {
		status = model.StatusFail
	} else if nearDestLoss >= 0.5 {
		status = model.StatusWarn
	}
	return model.CheckResult{
		ID:         c.ID(),
		Group:      c.Group(),
		Target:     c.Target,
		Status:     status,
		Metrics:    map[string]any{"hop_count": hops, "near_dest_loss_pct": nearDestLoss},
		Raw:        res.Stdout,
		Error:      stderrMsg(res.Stderr),
		DurationMS: time.Since(start).Milliseconds(),
	}
}

func (c PathCheck) runTraceroute(ctx context.Context, ex execx.Executor, timeoutSec int, start time.Time) model.CheckResult {
	if _, terr := ex.LookPath("traceroute"); terr != nil {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusSkip, Error: "mtr and traceroute not found"}
	}
	res := runWithTimeout(ctx, timeoutSec, ex, "traceroute", "-m", "15", c.Target)
	if isInterruptedError(res.Err) {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusFail, Error: res.Err.Error(), Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	if res.Err != nil && res.Stdout == "" {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusFail, Error: res.Err.Error(), DurationMS: time.Since(start).Milliseconds()}
	}
	status := model.StatusPass
	hops, timeoutHops := parseTracerouteSummary(res.Stdout)
	if timeoutHops > 0 {
		status = model.StatusWarn
	}
	return model.CheckResult{
		ID:         c.ID(),
		Group:      c.Group(),
		Target:     c.Target,
		Status:     status,
		Metrics:    map[string]any{"hop_count": hops, "timeout_hops": timeoutHops},
		Raw:        res.Stdout,
		Error:      stderrMsg(res.Stderr),
		DurationMS: time.Since(start).Milliseconds(),
	}
}
