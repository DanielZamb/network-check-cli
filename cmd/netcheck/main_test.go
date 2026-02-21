package main

import (
	"bytes"
	"context"
	"encoding/json"
	"netcheck/internal/execx"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeExecutor() *execx.FakeExecutor {
	return &execx.FakeExecutor{
		Paths: map[string]bool{"netstat": true, "ping": true, "dig": true, "curl": true, "mtr": true, "speedtest-cli": true, "iperf3": true},
		Outputs: map[string]execx.Result{
			"netstat -rn":         {Stdout: "default 10.0.0.1"},
			"ping -c 10 10.0.0.1": {Stdout: "10 packets transmitted, 10 packets received, 0.0% packet loss\nround-trip min/avg/max/stddev = 1.0/2.0/3.0/1.0 ms"},
			"ping -c 10 1.1.1.1":  {Stdout: "10 packets transmitted, 10 packets received, 0.0% packet loss\nround-trip min/avg/max/stddev = 1.0/2.0/3.0/1.0 ms"},
			"ping -c 10 8.8.8.8":  {Stdout: "10 packets transmitted, 10 packets received, 0.0% packet loss\nround-trip min/avg/max/stddev = 1.0/2.0/3.0/1.0 ms"},
			"dig google.com":      {Stdout: ";; Query time: 20 msec"},
			"curl -w dns:%{time_namelookup} connect:%{time_connect} tls:%{time_appconnect} ttfb:%{time_starttransfer} total:%{time_total} -o /dev/null -s https://example.com": {Stdout: "dns:0.01 connect:0.02 tls:0.03 ttfb:0.04 total:0.05"},
			"mtr -rwzc 10 1.1.1.1": {Stdout: "HOST: y"},
			"speedtest-cli --json": {Stdout: `{"download":100000000,"upload":50000000}`},
		},
	}
}

func TestRunCommandJSON(t *testing.T) {
	var out, errb bytes.Buffer
	ex := fakeExecutor()
	code := runCLI(context.Background(), []string{"run", "--format", "json", "--skip", "bandwidth"}, &out, &errb, ex)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["schema_version"] != "v1" {
		t.Fatalf("missing schema_version")
	}
}

func TestRunCommandJSONL(t *testing.T) {
	var out, errb bytes.Buffer
	ex := fakeExecutor()
	code := runCLI(context.Background(), []string{"run", "--format", "jsonl", "--skip", "bandwidth"}, &out, &errb, ex)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "\"schema_version\":\"v1\"") {
		t.Fatalf("unexpected jsonl output: %q", out.String())
	}
}

func TestManGolden(t *testing.T) {
	topics := []string{"", "run", "soak", "compare", "config", "exit-codes", "json-schema"}
	for _, topic := range topics {
		topic := topic
		t.Run("topic_"+strings.ReplaceAll(topic, "-", "_"), func(t *testing.T) {
			var out, errb bytes.Buffer
			args := []string{"man"}
			name := "netcheck"
			if topic != "" {
				args = append(args, topic)
				name = topic
			}
			code := runCLI(context.Background(), args, &out, &errb, fakeExecutor())
			if code != 0 {
				t.Fatalf("code=%d err=%s", code, errb.String())
			}
			gold := filepath.Join("..", "..", "testdata", "golden", "man", name+".txt")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				_ = os.WriteFile(gold, out.Bytes(), 0o644)
			}
			exp, err := os.ReadFile(gold)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(out.String()) != strings.TrimSpace(string(exp)) {
				t.Fatalf("mismatch for %s", name)
			}
		})
	}
}

func TestSoakEmitsJSONL(t *testing.T) {
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"soak", "--duration", "1", "--interval", "1", "--skip", "bandwidth"}, &out, &errb, fakeExecutor())
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "run_started") || !strings.Contains(out.String(), "check_result") {
		t.Fatalf("missing expected events")
	}
}

func TestSoakGlobalTimeout(t *testing.T) {
	var out, errb bytes.Buffer
	start := time.Now()
	code := runCLI(context.Background(), []string{"soak", "--duration", "10", "--interval", "1", "--skip", "bandwidth", "--timeout", "1"}, &out, &errb, fakeExecutor())
	if code != 0 && code != 1 {
		t.Fatalf("unexpected code=%d err=%s", code, errb.String())
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("soak did not respect global timeout; elapsed=%s", time.Since(start))
	}
	if !strings.Contains(out.String(), "run_finished") {
		t.Fatalf("expected run_finished event")
	}
}

func TestSoakUsesConfigDefaultsForIntervalDuration(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "netcheck.yaml")
	cfg := `soak:
  interval_sec: 1
  duration_sec: 1
targets:
  ping: ["1.1.1.1","8.8.8.8"]
  dns_domains: ["google.com"]
  http_urls: ["https://example.com"]
bandwidth:
  speedtest:
    enabled: false
  iperf:
    enabled: false
`
	_ = os.WriteFile(cfgPath, []byte(cfg), 0o644)
	var out, errb bytes.Buffer
	start := time.Now()
	code := runCLI(context.Background(), []string{"soak", "--config", cfgPath}, &out, &errb, fakeExecutor())
	if code != 0 && code != 1 {
		t.Fatalf("unexpected code=%d err=%s", code, errb.String())
	}
	if time.Since(start) > 4*time.Second {
		t.Fatalf("soak ignored config duration, elapsed=%s", time.Since(start))
	}
	if !strings.Contains(out.String(), "interval_summary") {
		t.Fatalf("expected interval_summary in output")
	}
}

func TestCompareJSON(t *testing.T) {
	d := t.TempDir()
	b1 := []byte(`{"score":10,"checks":[{"id":"a","status":"warn","duration_ms":1}]}`)
	b2 := []byte(`{"score":20,"checks":[{"id":"a","status":"pass","duration_ms":2}]}`)
	p1 := filepath.Join(d, "b.json")
	p2 := filepath.Join(d, "c.json")
	_ = os.WriteFile(p1, b1, 0o644)
	_ = os.WriteFile(p2, b2, 0o644)
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"compare", "--format", "json", p1, p2}, &out, &errb, fakeExecutor())
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "before_score") {
		t.Fatal("missing compare json")
	}
}

func TestCompareJSONOutError(t *testing.T) {
	d := t.TempDir()
	b1 := []byte(`{"score":10,"checks":[{"id":"a","status":"warn","duration_ms":1}]}`)
	b2 := []byte(`{"score":20,"checks":[{"id":"a","status":"pass","duration_ms":2}]}`)
	p1 := filepath.Join(d, "b.json")
	p2 := filepath.Join(d, "c.json")
	_ = os.WriteFile(p1, b1, 0o644)
	_ = os.WriteFile(p2, b2, 0o644)
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"compare", "--format", "json", "--out", "/no/such/dir/out.json", p1, p2}, &out, &errb, fakeExecutor())
	if code != 4 {
		t.Fatalf("expected output error, got %d", code)
	}
}

func TestRunInvalidFormatExitCode(t *testing.T) {
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"run", "--format", "nope"}, &out, &errb, fakeExecutor())
	if code != 4 {
		t.Fatalf("expected output error 4, got %d", code)
	}
}

func TestRunVerboseWritesProgress(t *testing.T) {
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"run", "--format", "json", "--skip", "bandwidth", "--verbose"}, &out, &errb, fakeExecutor())
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "op : starting check") {
		t.Fatalf("expected verbose check log, got: %s", errb.String())
	}
}

func TestRunStrictWarn(t *testing.T) {
	var out, errb bytes.Buffer
	ex := fakeExecutor()
	ex.Outputs["dig google.com"] = execx.Result{Stdout: ";; Query time: 999 msec"}
	code := runCLI(context.Background(), []string{"run", "--format", "json", "--skip", "bandwidth", "--strict-warn"}, &out, &errb, ex)
	if code != 1 {
		t.Fatalf("expected checks-failed code=1, got %d", code)
	}
}

func TestManUnknownTopic(t *testing.T) {
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"man", "unknown-topic"}, &out, &errb, fakeExecutor())
	if code != 2 {
		t.Fatalf("expected config error, got %d", code)
	}
	if !strings.Contains(errb.String(), "available topics") {
		t.Fatalf("missing topics hint")
	}
}

func TestRunOutputPathError(t *testing.T) {
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"run", "--format", "json", "--skip", "bandwidth", "--out", "/no/such/dir/report.json"}, &out, &errb, fakeExecutor())
	if code != 4 {
		t.Fatalf("expected output error, got %d", code)
	}
}

func TestRunTimeoutExemptionForLongChecks(t *testing.T) {
	ex := fakeExecutor()
	ex.Delays = map[string]time.Duration{
		"speedtest-cli --json": 1500 * time.Millisecond,
	}
	var out, errb bytes.Buffer
	start := time.Now()
	code := runCLI(context.Background(), []string{"run", "--format", "json", "--select", "bandwidth", "--timeout", "1"}, &out, &errb, ex)
	if code != 0 {
		t.Fatalf("expected success, got code=%d err=%s out=%s", code, errb.String(), out.String())
	}
	if time.Since(start) < 1200*time.Millisecond {
		t.Fatalf("expected timeout exemption to allow delayed speedtest; elapsed=%s", time.Since(start))
	}
}
