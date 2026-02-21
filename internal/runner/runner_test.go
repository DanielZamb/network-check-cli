package runner

import (
	"context"
	"netcheck/internal/config"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"strings"
	"testing"
)

func TestRunOnceFakeExecutor(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bandwidth.Speedtest.Enabled = false
	cfg.Bandwidth.Iperf.Enabled = false
	cfg.Targets.Resolvers = []string{"1.1.1.1"}
	fake := &execx.FakeExecutor{
		Paths: map[string]bool{"netstat": true, "ping": true, "dig": true, "curl": true, "mtr": true},
		Outputs: map[string]execx.Result{
			"netstat -rn":             {Stdout: "Routing tables\ndefault 10.0.0.1"},
			"ping -c 10 10.0.0.1":     {Stdout: samplePing()},
			"ping -c 10 1.1.1.1":      {Stdout: samplePing()},
			"ping -c 10 8.8.8.8":      {Stdout: samplePing()},
			"dig google.com":          {Stdout: ";; Query time: 20 msec"},
			"dig @1.1.1.1 google.com": {Stdout: ";; Query time: 22 msec"},
			"curl -w dns:%{time_namelookup} connect:%{time_connect} tls:%{time_appconnect} ttfb:%{time_starttransfer} total:%{time_total} -o /dev/null -s https://example.com": {Stdout: "dns:0.01 connect:0.02 tls:0.03 ttfb:0.04 total:0.10"},
			"mtr -rwzc 10 1.1.1.1": {Stdout: "HOST: x"},
		},
	}
	r, err := RunOnce(context.Background(), fake, cfg, model.RunOptions{}, "dev", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.SchemaVersion == "" || r.Report.Score == 0 {
		t.Fatalf("unexpected report")
	}
}

func TestRunOnceSelectSkip(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bandwidth.Speedtest.Enabled = false
	cfg.Bandwidth.Iperf.Enabled = false
	fake := &execx.FakeExecutor{
		Paths: map[string]bool{"dig": true},
		Outputs: map[string]execx.Result{
			"dig google.com": {Stdout: ";; Query time: 20 msec"},
		},
	}
	r, err := RunOnce(context.Background(), fake, cfg, model.RunOptions{Select: []string{"dns"}, Skip: []string{"bandwidth"}}, "dev", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Report.Checks) != 1 || r.Report.Checks[0].Group != "dns" {
		t.Fatalf("unexpected checks: %+v", r.Report.Checks)
	}
}

func samplePing() string {
	return strings.Join([]string{
		"10 packets transmitted, 10 packets received, 0.0% packet loss",
		"round-trip min/avg/max/stddev = 10.000/20.000/30.000/5.000 ms",
	}, "\n")
}
