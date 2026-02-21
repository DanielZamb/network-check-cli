package checks

import (
	"context"
	"errors"
	"netcheck/internal/config"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"testing"
	"time"
)

func cfg() config.Config {
	c := config.Defaults()
	c.ExpectedPlan.DownloadMbps = 100
	c.ExpectedPlan.UploadMbps = 50
	c.Bandwidth.Iperf.Target = "10.0.0.2"
	c.Targets.Resolvers = []string{"1.1.1.1"}
	return c
}

func pingOK() string {
	return "10 packets transmitted, 10 packets received, 0.0% packet loss\nround-trip min/avg/max/stddev = 1.000/2.000/3.000/1.000 ms"
}

func TestLocalCheckIncludesInterfaceMetrics(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"netstat": true, "ping": true, "ifconfig": true}, Outputs: map[string]execx.Result{
		"netstat -rn":         {Stdout: "default 10.0.0.1"},
		"ping -c 10 10.0.0.1": {Stdout: pingOK()},
		"ifconfig":            {Stdout: "en0:\n\tstatus: active\n\tinet 10.0.0.5\n"},
	}}
	r := LocalCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusPass {
		t.Fatalf("expected pass, got %s", r.Status)
	}
	if _, ok := r.Metrics["active_interfaces"]; !ok {
		t.Fatalf("missing active_interfaces metric")
	}
}

func TestDNSCheckWithResolver(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"dig": true}, Outputs: map[string]execx.Result{
		"dig @1.1.1.1 google.com": {Stdout: ";; Query time: 12 msec"},
	}}
	r := DNSCheck{Domain: "google.com", Resolver: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusPass {
		t.Fatalf("expected pass, got %s", r.Status)
	}
	if r.Target != "google.com via 1.1.1.1" {
		t.Fatalf("unexpected target: %s", r.Target)
	}
}

func TestPathCheckTracerouteFallback(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"traceroute": true}, Outputs: map[string]execx.Result{
		"traceroute -m 15 1.1.1.1": {Stdout: "traceroute to 1.1.1.1\n"},
	}}
	r := PathCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusPass {
		t.Fatalf("expected pass, got %s", r.Status)
	}
}

func TestPathCheckFallsBackWhenMTRRuntimeFails(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{
		Paths: map[string]bool{"mtr": true, "traceroute": true},
		Outputs: map[string]execx.Result{
			"mtr -rwzc 10 1.1.1.1":     {Err: errors.New("exit status 1"), ExitCode: 1},
			"traceroute -m 15 1.1.1.1": {Stdout: "traceroute to 1.1.1.1\n 1 a 1.0 ms\n"},
		},
	}
	r := PathCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusPass {
		t.Fatalf("expected pass after traceroute fallback, got %s (err=%s)", r.Status, r.Error)
	}
}

func TestIperfUnreachableIsSkip(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"iperf3": true}, Outputs: map[string]execx.Result{
		"iperf3 -c 10.0.0.2 -P 4 -t 30 -J": {Err: errors.New("unable to connect"), Stderr: "unable to connect", ExitCode: 1},
	}}
	r := IperfCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusSkip {
		t.Fatalf("expected skip, got %s", r.Status)
	}
}

func TestBufferbloatUsesSpeedtestLoadProxy(t *testing.T) {
	c := cfg()
	c.Bandwidth.Iperf.Target = ""
	fx := &execx.FakeExecutor{Paths: map[string]bool{"ping": true, "speedtest-cli": true}, Outputs: map[string]execx.Result{
		"ping -c 10 1.1.1.1":   {Stdout: pingOK()},
		"speedtest-cli --json": {Stdout: `{"download":100000000,"upload":50000000}`},
	}}
	r := BufferbloatCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status == "" {
		t.Fatalf("expected status")
	}
	seen := false
	for _, call := range fx.Calls {
		if call == "speedtest-cli --json" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("expected speedtest load call")
	}
}

func TestReachabilityTimeout(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{
		Paths: map[string]bool{"ping": true},
		Delays: map[string]time.Duration{
			"ping -c 10 1.1.1.1": 2 * time.Second,
		},
	}
	r := ReachabilityCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 1)
	if r.Status != model.StatusFail {
		t.Fatalf("expected fail, got %s", r.Status)
	}
}

func TestReachabilityUsesP95Threshold(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{
		Paths: map[string]bool{"ping": true},
		Outputs: map[string]execx.Result{
			"ping -c 10 1.1.1.1": {Stdout: "64 bytes from 1.1.1.1: icmp_seq=0 ttl=57 time=5.0 ms\n64 bytes from 1.1.1.1: icmp_seq=1 ttl=57 time=6.0 ms\n64 bytes from 1.1.1.1: icmp_seq=2 ttl=57 time=120.0 ms\n10 packets transmitted, 10 packets received, 0.0% packet loss\nround-trip min/avg/max/stddev = 5.000/10.000/120.000/1.000 ms"},
		},
	}
	r := ReachabilityCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusFail {
		t.Fatalf("expected fail from high p95, got %s", r.Status)
	}
}

func TestSpeedtestWarnOnInvalidJSON(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"speedtest-cli": true}, Outputs: map[string]execx.Result{
		"speedtest-cli --json": {Stdout: "not-json"},
	}}
	r := SpeedtestCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusWarn {
		t.Fatalf("expected warn, got %s", r.Status)
	}
}

func TestIperfMissingTargetSkip(t *testing.T) {
	c := cfg()
	c.Bandwidth.Iperf.Target = ""
	fx := &execx.FakeExecutor{}
	r := IperfCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusSkip {
		t.Fatalf("expected skip, got %s", r.Status)
	}
}

func TestPathCheckSkipWhenNoTools(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{}}
	r := PathCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusSkip {
		t.Fatalf("expected skip, got %s", r.Status)
	}
}

func TestDNSMissingToolSkip(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{}}
	r := DNSCheck{Domain: "google.com"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusSkip {
		t.Fatalf("expected skip, got %s", r.Status)
	}
}

func TestHTTPMissingToolSkip(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{}}
	r := HTTPCheck{URL: "https://example.com"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusSkip {
		t.Fatalf("expected skip, got %s", r.Status)
	}
}

func TestLocalGatewayWarnWhenMissing(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"netstat": true}, Outputs: map[string]execx.Result{
		"netstat -rn": {Stdout: "Routing tables\n"},
	}}
	r := LocalCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusWarn {
		t.Fatalf("expected warn, got %s", r.Status)
	}
}

func TestPathMTRLossFail(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"mtr": true}, Outputs: map[string]execx.Result{
		"mtr -rwzc 10 1.1.1.1": {Stdout: "1.|-- hop-a 0.0%\n2.|-- hop-b 5.0%"},
	}}
	r := PathCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusFail {
		t.Fatalf("expected fail, got %s", r.Status)
	}
}

func TestSpeedtestThresholdFail(t *testing.T) {
	c := cfg()
	c.ExpectedPlan.DownloadMbps = 500
	fx := &execx.FakeExecutor{Paths: map[string]bool{"speedtest-cli": true}, Outputs: map[string]execx.Result{
		"speedtest-cli --json": {Stdout: `{"download":100000000,"upload":50000000}`},
	}}
	r := SpeedtestCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status != model.StatusFail {
		t.Fatalf("expected fail, got %s", r.Status)
	}
}

func TestIperfParsePass(t *testing.T) {
	c := cfg()
	fx := &execx.FakeExecutor{Paths: map[string]bool{"iperf3": true}, Outputs: map[string]execx.Result{
		"iperf3 -c 10.0.0.2 -P 4 -t 30 -J": {Stdout: `{"end":{"sum_received":{"bits_per_second":160000000}}}`},
	}}
	r := IperfCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status == model.StatusFail || r.Metrics["download_mbps"] == nil {
		t.Fatalf("expected throughput metric and non-fail status, got %+v", r)
	}
}

func TestIperfTargetWithPortUsesPortFlag(t *testing.T) {
	c := cfg()
	c.Bandwidth.Iperf.Target = "10.0.0.2:5201"
	fx := &execx.FakeExecutor{Paths: map[string]bool{"iperf3": true}, Outputs: map[string]execx.Result{
		"iperf3 -c 10.0.0.2 -p 5201 -P 4 -t 30 -J": {Stdout: `{"end":{"sum_received":{"bits_per_second":120000000}}}`},
	}}
	r := IperfCheck{}.Run(context.Background(), fx, c, 2)
	if r.Status == model.StatusSkip || r.Status == model.StatusFail {
		t.Fatalf("expected non-fail status, got %s", r.Status)
	}
}

func TestBufferbloatIperfTargetWithPort(t *testing.T) {
	c := cfg()
	c.Bandwidth.Iperf.Target = "10.0.0.2:5201"
	fx := &execx.FakeExecutor{Paths: map[string]bool{"ping": true, "iperf3": true}, Outputs: map[string]execx.Result{
		"ping -c 10 1.1.1.1":                   {Stdout: pingOK()},
		"iperf3 -c 10.0.0.2 -p 5201 -P 2 -t 5": {Stdout: "ok"},
	}}
	r := BufferbloatCheck{Target: "1.1.1.1"}.Run(context.Background(), fx, c, 2)
	if r.Status == "" {
		t.Fatalf("expected status")
	}
}
