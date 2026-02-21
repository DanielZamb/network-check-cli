package output

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"netcheck/internal/model"
)

func sampleReport() model.Report {
	return model.Report{
		SchemaVersion: "v1",
		Timestamp:     time.Unix(0, 0).UTC(),
		Host:          "host",
		OS:            "darwin",
		Version:       "dev",
		Config: map[string]any{
			"expected_plan": map[string]any{"download_mbps": float64(300), "upload_mbps": float64(30)},
			"thresholds": map[string]any{
				"dns_pass_max_ms":              float64(50),
				"dns_warn_max_ms":              float64(120),
				"http_pass_max_ms":             float64(800),
				"http_warn_max_ms":             float64(2000),
				"loss_pass_max":                float64(0.5),
				"rtt_p95_pass_max_ms":          float64(40),
				"jitter_pass_max_ms":           float64(10),
				"loaded_latency_pass_delta_ms": float64(30),
				"throughput_pass_pct":          float64(80),
			},
		},
		Checks: []model.CheckResult{
			{ID: "bandwidth.speedtest", Group: "bandwidth", Status: model.StatusWarn, Metrics: map[string]any{"download_mbps": float64(120), "upload_mbps": float64(20)}},
			{ID: "dns.google.com", Group: "dns", Status: model.StatusPass, Metrics: map[string]any{"query_ms": float64(22)}},
		},
		Summary: model.Summary{Pass: 1, Warn: 1, Total: 2},
		Score:   100,
	}
}

func TestTableGolden(t *testing.T) {
	r := sampleReport()
	var b bytes.Buffer
	if err := WriteTable(&b, r); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	goldPath := filepath.Join("..", "..", "testdata", "golden", "table.txt")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.WriteFile(goldPath, []byte(got), 0o644)
	}
	exp, err := os.ReadFile(goldPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != strings.TrimSpace(string(exp)) {
		t.Fatalf("table mismatch\n---got---\n%s\n---exp---\n%s", got, string(exp))
	}
}

func TestJSONGolden(t *testing.T) {
	r := sampleReport()
	got, err := JSONBytes(r)
	if err != nil {
		t.Fatal(err)
	}
	goldPath := filepath.Join("..", "..", "testdata", "golden", "report.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.WriteFile(goldPath, got, 0o644)
	}
	exp, err := os.ReadFile(goldPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(string(exp)) {
		t.Fatalf("json mismatch")
	}
}

func TestTableString(t *testing.T) {
	r := sampleReport()
	s, err := TableString(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "Summary") {
		t.Fatalf("unexpected table string")
	}
	if !strings.Contains(s, "Group Summary") {
		t.Fatalf("missing group summary")
	}
	if !strings.Contains(s, "dl=120.0Mbps ul=20.0Mbps") {
		t.Fatalf("missing measured bandwidth metric")
	}
	if !strings.Contains(s, "dl>=300.0Mbps ul>=30.0Mbps pass@80%") {
		t.Fatalf("missing expected bandwidth metric")
	}
}

func TestWriteTableWithColor(t *testing.T) {
	r := sampleReport()
	var b bytes.Buffer
	if err := WriteTableWithOptions(&b, r, TableOptions{Color: true}); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	if !strings.Contains(s, "\u001b[32m") {
		t.Fatalf("expected ANSI status color in output")
	}
	if !strings.Contains(s, "\x1b[38;5;214mbandwidth\x1b[0m") {
		t.Fatalf("expected ANSI group color in check rows")
	}
	if !strings.Contains(s, "\x1b[38;5;33mdns\x1b[0m") {
		t.Fatalf("expected ANSI group color in group summary rows")
	}
}

func TestPathMeasuredUsesTracerouteMetrics(t *testing.T) {
	r := sampleReport()
	r.Checks = append(r.Checks, model.CheckResult{
		ID:     "path.1.1.1.1",
		Group:  "path",
		Status: model.StatusPass,
		Metrics: map[string]any{
			"hop_count":    float64(12),
			"timeout_hops": float64(1),
		},
	})
	var b bytes.Buffer
	if err := WriteTable(&b, r); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	if !strings.Contains(s, "hops=12 timeout_hops=1") {
		t.Fatalf("expected traceroute-style path metrics, got: %s", s)
	}
}
