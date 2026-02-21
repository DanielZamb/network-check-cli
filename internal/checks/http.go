package checks

import (
	"context"
	"net"
	"net/url"
	"netcheck/internal/config"
	"netcheck/internal/eval"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"strings"
	"time"
)

type HTTPCheck struct{ URL string }

func (c HTTPCheck) ID() string    { return "http." + c.URL }
func (c HTTPCheck) Group() string { return "http" }

func (c HTTPCheck) Run(ctx context.Context, ex execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult {
	start := time.Now()
	if _, err := ex.LookPath("curl"); err != nil {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.URL, Status: model.StatusSkip, Error: "curl not found"}
	}
	res := runWithTimeout(ctx, timeoutSec, ex, "curl", "-w", "dns:%{time_namelookup} connect:%{time_connect} tls:%{time_appconnect} ttfb:%{time_starttransfer} total:%{time_total}", "-o", "/dev/null", "-s", c.URL)
	if isInterruptedError(res.Err) {
		return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.URL, Status: model.StatusFail, Error: res.Err.Error(), Raw: res.Stdout, DurationMS: time.Since(start).Milliseconds()}
	}
	m := parseCurlTimings(res.Stdout)
	total := m["total"]
	status := eval.LowerIsBetter(total, cfg.Thresholds.HTTPPassMaxMs, cfg.Thresholds.HTTPWarnMaxMs)
	metrics := map[string]any{"dns_ms": m["dns"], "connect_ms": m["connect"], "tls_ms": m["tls"], "ttfb_ms": m["ttfb"], "total_ms": total}
	if _, err := ex.LookPath("openssl"); err == nil {
		if u, err := url.Parse(c.URL); err == nil {
			host := u.Host
			if !strings.Contains(host, ":") {
				host = net.JoinHostPort(host, "443")
			}
			meta := runWithTimeout(ctx, timeoutSec, ex, "openssl", "s_client", "-connect", host, "-servername", u.Hostname())
			if meta.Stdout != "" {
				for _, line := range strings.Split(meta.Stdout, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Protocol") {
						metrics["tls_protocol"] = strings.TrimSpace(strings.TrimPrefix(line, "Protocol  :"))
					}
					if strings.HasPrefix(line, "Cipher") {
						metrics["tls_cipher"] = strings.TrimSpace(strings.TrimPrefix(line, "Cipher    :"))
					}
				}
			}
		}
	}
	return model.CheckResult{ID: c.ID(), Group: c.Group(), Target: c.URL, Status: status, Metrics: metrics, Raw: res.Stdout, Error: stderrMsg(res.Stderr), DurationMS: time.Since(start).Milliseconds()}
}
