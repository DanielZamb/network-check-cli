package checks

import (
	"context"
	"encoding/json"
	"net"
	"netcheck/internal/config"
	"netcheck/internal/eval"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"strconv"
	"strings"
	"time"
)

type SpeedtestCheck struct{}

type IperfCheck struct{}

func (SpeedtestCheck) ID() string    { return "bandwidth.speedtest" }
func (SpeedtestCheck) Group() string { return "bandwidth" }
func (IperfCheck) ID() string        { return "bandwidth.iperf" }
func (IperfCheck) Group() string     { return "bandwidth" }

func (SpeedtestCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if !cfg.Bandwidth.Speedtest.Enabled {
		return model.CheckResult{ID: "bandwidth.speedtest", Group: "bandwidth", Status: model.StatusSkip, Error: "speedtest disabled"}
	}
	if _, err := ex.LookPath("speedtest-cli"); err != nil {
		return model.CheckResult{ID: "bandwidth.speedtest", Group: "bandwidth", Status: model.StatusSkip, Error: "speedtest-cli not found"}
	}
	args := []string{"--json"}
	if cfg.Bandwidth.Speedtest.ServerID != "" {
		args = append(args, "--server", cfg.Bandwidth.Speedtest.ServerID)
	}
	localTimeout := timeoutSec
	if localTimeout < 45 {
		localTimeout = 45
	}
	res := runWithTimeout(ctx, localTimeout, ex, "speedtest-cli", args...)
	if isInterruptedError(res.Err) {
		return model.CheckResult{ID: "bandwidth.speedtest", Group: "bandwidth", Status: model.StatusFail, Error: res.Err.Error(), Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	if res.Err != nil && res.Stdout == "" {
		return model.CheckResult{ID: "bandwidth.speedtest", Group: "bandwidth", Status: model.StatusFail, Error: res.Err.Error(), DurationMS: time.Since(start).Milliseconds()}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &obj); err != nil {
		return model.CheckResult{ID: "bandwidth.speedtest", Group: "bandwidth", Status: model.StatusWarn, Error: "unable to parse speedtest json", Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	dl := toMbps(obj["download"])
	ul := toMbps(obj["upload"])
	status := model.StatusPass
	errMsg := ""
	metrics := map[string]any{"download_mbps": dl, "upload_mbps": ul}
	if cfg.ExpectedPlan.DownloadMbps > 0 {
		dlPct := dl / cfg.ExpectedPlan.DownloadMbps * 100
		metrics["download_pct_of_expected"] = dlPct
		status = eval.UpperIsBetter(dlPct, cfg.Thresholds.ThroughputPassPct, cfg.Thresholds.ThroughputWarnPct)
	}
	if status == model.StatusPass && cfg.ExpectedPlan.UploadMbps > 0 {
		ulPct := ul / cfg.ExpectedPlan.UploadMbps * 100
		metrics["upload_pct_of_expected"] = ulPct
		status = eval.UpperIsBetter(ulPct, cfg.Thresholds.ThroughputPassPct, cfg.Thresholds.ThroughputWarnPct)
	}
	if status != model.StatusPass && (cfg.ExpectedPlan.DownloadMbps > 0 || cfg.ExpectedPlan.UploadMbps > 0) {
		errMsg = "throughput below expected plan thresholds"
	}
	return model.CheckResult{ID: "bandwidth.speedtest", Group: "bandwidth", Status: status, Metrics: metrics, Error: errMsg, Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
}

func (IperfCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if !cfg.Bandwidth.Iperf.Enabled {
		return model.CheckResult{ID: "bandwidth.iperf", Group: "bandwidth", Status: model.StatusSkip, Error: "iperf disabled"}
	}
	if cfg.Bandwidth.Iperf.Target == "" {
		return model.CheckResult{ID: "bandwidth.iperf", Group: "bandwidth", Status: model.StatusSkip, Error: "iperf target not configured"}
	}
	if _, err := ex.LookPath("iperf3"); err != nil {
		return model.CheckResult{ID: "bandwidth.iperf", Group: "bandwidth", Status: model.StatusSkip, Error: "iperf3 not found"}
	}
	args := buildIperfClientArgs(cfg.Bandwidth.Iperf.Target, cfg.Bandwidth.Iperf.ParallelStreams, cfg.Bandwidth.Iperf.DurationSec, true)
	localTimeout := timeoutSec
	minIperfTimeout := cfg.Bandwidth.Iperf.DurationSec + 10
	if localTimeout < minIperfTimeout {
		localTimeout = minIperfTimeout
	}
	res := runWithTimeout(ctx, localTimeout, ex, "iperf3", args...)
	if isInterruptedError(res.Err) {
		return model.CheckResult{ID: "bandwidth.iperf", Group: "bandwidth", Target: cfg.Bandwidth.Iperf.Target, Status: model.StatusFail, Error: res.Err.Error(), Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	if res.Err != nil && res.Stdout == "" {
		if isIperfUnreachable(res.Err.Error(), res.Stderr) {
			return model.CheckResult{ID: "bandwidth.iperf", Group: "bandwidth", Target: cfg.Bandwidth.Iperf.Target, Status: model.StatusSkip, Error: "iperf target unreachable", DurationMS: time.Since(start).Milliseconds()}
		}
		return model.CheckResult{ID: "bandwidth.iperf", Group: "bandwidth", Target: cfg.Bandwidth.Iperf.Target, Status: model.StatusFail, Error: res.Err.Error(), DurationMS: time.Since(start).Milliseconds()}
	}
	mbps := parseIperfMbps(res.Stdout)
	status := model.StatusPass
	errMsg := ""
	metrics := map[string]any{"download_mbps": mbps}
	if cfg.ExpectedPlan.DownloadMbps > 0 {
		pct := mbps / cfg.ExpectedPlan.DownloadMbps * 100
		metrics["download_pct_of_expected"] = pct
		status = eval.UpperIsBetter(pct, cfg.Thresholds.ThroughputPassPct, cfg.Thresholds.ThroughputWarnPct)
	}
	if status != model.StatusPass && cfg.ExpectedPlan.DownloadMbps > 0 {
		errMsg = "throughput below expected plan thresholds"
	}
	return model.CheckResult{ID: "bandwidth.iperf", Group: "bandwidth", Target: cfg.Bandwidth.Iperf.Target, Status: status, Metrics: metrics, Error: errMsg, Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
}

func toMbps(v any) float64 {
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return f / 1_000_000
}

func parseIperfMbps(raw string) float64 {
	marker := `"bits_per_second":`
	idx := strings.LastIndex(raw, marker)
	if idx < 0 {
		return 0
	}
	rest := raw[idx+len(marker):]
	rest = strings.TrimSpace(rest)
	end := strings.IndexAny(rest, ",}\n")
	if end < 0 {
		end = len(rest)
	}
	val := strings.TrimSpace(rest[:end])
	f, _ := strconv.ParseFloat(val, 64)
	return f / 1_000_000
}

func buildIperfClientArgs(target string, parallelStreams, durationSec int, jsonOut bool) []string {
	host := target
	port := ""
	// net.SplitHostPort handles bracketed IPv6 and host:port.
	if h, p, err := net.SplitHostPort(target); err == nil {
		host = h
		port = p
	} else if strings.Count(target, ":") == 1 {
		parts := strings.SplitN(target, ":", 2)
		if _, err := strconv.Atoi(parts[1]); err == nil {
			host = parts[0]
			port = parts[1]
		}
	}
	args := []string{"-c", host}
	if port != "" {
		args = append(args, "-p", port)
	}
	args = append(args, "-P", strconv.Itoa(parallelStreams), "-t", strconv.Itoa(durationSec))
	if jsonOut {
		args = append(args, "-J")
	}
	return args
}

func isIperfUnreachable(errMsg, stderr string) bool {
	s := strings.ToLower(errMsg + " " + stderr)
	return strings.Contains(s, "unable to connect") ||
		strings.Contains(s, "no route to host") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "network is unreachable") ||
		strings.Contains(s, "timed out")
}
