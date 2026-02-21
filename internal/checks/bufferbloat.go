package checks

import (
	"context"
	"netcheck/internal/config"
	"netcheck/internal/eval"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"time"
)

type BufferbloatCheck struct{ Target string }

func (c BufferbloatCheck) ID() string    { return "bufferbloat." + c.Target }
func (c BufferbloatCheck) Group() string { return "bufferbloat" }

func (c BufferbloatCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if _, err := ex.LookPath("ping"); err != nil {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusSkip, Error: "ping not found"}
	}
	idle := runWithTimeout(ctx, timeoutSec, ex, "ping", "-c", "10", c.Target)
	if isInterruptedError(idle.Err) {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusFail, Error: idle.Err.Error(), Raw: idle.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	_, idleAvg, _, _ := parsePing(idle.Stdout)
	// Load proxy: run one bandwidth command between baseline and loaded latency samples.
	if cfg.Bandwidth.Iperf.Enabled && cfg.Bandwidth.Iperf.Target != "" {
		args := buildIperfClientArgs(cfg.Bandwidth.Iperf.Target, 2, 5, false)
		_ = runWithTimeout(ctx, timeoutSec, ex, "iperf3", args...)
	} else if cfg.Bandwidth.Speedtest.Enabled {
		_ = runWithTimeout(ctx, timeoutSec, ex, "speedtest-cli", "--json")
	}
	loaded := runWithTimeout(ctx, timeoutSec, ex, "ping", "-c", "10", c.Target)
	if isInterruptedError(loaded.Err) {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: model.StatusFail, Error: loaded.Err.Error(), Raw: loaded.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	_, loadedAvg, _, _ := parsePing(loaded.Stdout)
	delta := loadedAvg - idleAvg
	status := eval.LowerIsBetter(delta, cfg.Thresholds.LoadedLatencyPassDeltaMs, cfg.Thresholds.LoadedLatencyWarnDeltaMs)
	return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Target, Status: status, Metrics: map[string]any{"idle_ms": idleAvg, "loaded_ms": loadedAvg, "delta_ms": delta}, Raw: loaded.Stdout, DurationMS: time.Since(start).Milliseconds()}
}
