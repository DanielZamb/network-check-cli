package checks

import (
	"context"
	"netcheck/internal/config"
	"netcheck/internal/eval"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"strings"
	"time"
)

type LocalCheck struct{}

func (LocalCheck) ID() string    { return "local.gateway" }
func (LocalCheck) Group() string { return "local" }

func (LocalCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if _, err := ex.LookPath("netstat"); err != nil {
		return model.CheckResult{ID: "local.gateway", Group: "local", Status: model.StatusSkip, Error: "netstat not found"}
	}
	res := runWithTimeout(ctx, timeoutSec, ex, "netstat", "-rn")
	if res.Err != nil {
		return model.CheckResult{ID: "local.gateway", Group: "local", Status: model.StatusFail, Error: res.Err.Error(), DurationMS: time.Since(start).Milliseconds()}
	}
	gw := ""
	for _, line := range strings.Split(res.Stdout, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "default") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				gw = fields[1]
				break
			}
		}
	}
	if gw == "" {
		return model.CheckResult{ID: "local.gateway", Group: "local", Status: model.StatusWarn, Error: "default gateway not detected", DurationMS: time.Since(start).Milliseconds()}
	}
	ping := runWithTimeout(ctx, timeoutSec, ex, "ping", "-c", "10", gw)
	if isInterruptedError(ping.Err) {
		return model.CheckResult{ID: "local.gateway", Group: "local", Target: gw, Status: model.StatusFail, Error: ping.Err.Error(), Raw: ping.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	loss, avg, jitter, _ := parsePing(ping.Stdout)
	status := eval.LowerIsBetter(loss, cfg.Thresholds.LossPassMax, cfg.Thresholds.LossWarnMax)
	metrics := map[string]any{"loss_pct": loss, "avg_ms": avg, "jitter_ms": jitter}
	if _, err := ex.LookPath("ifconfig"); err == nil {
		ifcfg := runWithTimeout(ctx, timeoutSec, ex, "ifconfig")
		lines := strings.Split(ifcfg.Stdout, "\n")
		active := 0
		ipCount := 0
		for _, l := range lines {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, "status: active") {
				active++
			}
			if strings.HasPrefix(t, "inet ") && !strings.Contains(t, "127.0.0.1") {
				ipCount++
			}
		}
		metrics["active_interfaces"] = active
		metrics["local_ip_count"] = ipCount
	}
	return model.CheckResult{ID: "local.gateway", Group: "local", Target: gw, Status: status, Metrics: metrics, Raw: ping.Stdout, DurationMS: time.Since(start).Milliseconds()}
}
