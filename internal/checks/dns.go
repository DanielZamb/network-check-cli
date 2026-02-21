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

type DNSCheck struct {
	Domain   string
	Resolver string
}

func (c DNSCheck) ID() string {
	if c.Resolver == "" {
		return "dns." + c.Domain
	}
	return "dns." + c.Domain + "@" + c.Resolver
}
func (c DNSCheck) Group() string { return "dns" }

func (c DNSCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if _, err := ex.LookPath("dig"); err != nil {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.Domain, Status: model.StatusSkip, Error: "dig not found"}
	}
	args := []string{c.Domain}
	target := c.Domain
	if c.Resolver != "" {
		args = []string{"@" + c.Resolver, c.Domain}
		target = c.Domain + " via " + c.Resolver
	}
	res := runWithTimeout(ctx, timeoutSec, ex, "dig", args...)
	if isInterruptedError(res.Err) {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: target, Status: model.StatusFail, Error: res.Err.Error(), Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	if res.Err != nil && res.Stdout == "" {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: target, Status: model.StatusFail, Error: res.Err.Error(), DurationMS: time.Since(start).Milliseconds()}
	}
	ms := parseDigMS(res.Stdout)
	status := eval.LowerIsBetter(ms, cfg.Thresholds.DNSPassMaxMs, cfg.Thresholds.DNSWarnMaxMs)
	if ms == 0 {
		status = model.StatusWarn
	}
	return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: target, Status: status, Metrics: map[string]any{"query_ms": ms}, Raw: res.Stdout, Error: fmt.Sprintf("%s", stderrMsg(res.Stderr)), DurationMS: time.Since(start).Milliseconds()}
}
