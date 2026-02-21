//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"netcheck/internal/execx"
)

func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("missing binary %s: %v", name, err)
	}
}

func TestIntegrationRunReachabilitySmoke(t *testing.T) {
	requireBinary(t, "ping")
	var out, errb bytes.Buffer
	code := runCLI(context.Background(), []string{"run", "--format", "json", "--select", "reachability", "--timeout", "10"}, &out, &errb, execx.RealExecutor{})
	if code != 0 && code != 1 {
		t.Fatalf("unexpected exit code=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "schema_version") {
		t.Fatalf("expected JSON report output")
	}
}

func TestIntegrationSoakRespectsGlobalTimeout(t *testing.T) {
	requireBinary(t, "ping")
	var out, errb bytes.Buffer
	start := time.Now()
	code := runCLI(context.Background(), []string{"soak", "--format", "jsonl", "--select", "reachability", "--duration", "30", "--interval", "1", "--timeout", "2"}, &out, &errb, execx.RealExecutor{})
	if code != 0 && code != 1 {
		t.Fatalf("unexpected exit code=%d stderr=%s", code, errb.String())
	}
	if time.Since(start) > 6*time.Second {
		t.Fatalf("soak timeout not respected, elapsed=%s", time.Since(start))
	}
	if !strings.Contains(out.String(), "run_finished") {
		t.Fatalf("expected run_finished event")
	}
}

func TestIntegrationCompareSmoke(t *testing.T) {
	var out, errb bytes.Buffer
	ex := execx.RealExecutor{}

	dir := t.TempDir()
	before := filepath.Join(dir, "before.json")
	after := filepath.Join(dir, "after.json")

	code := runCLI(context.Background(), []string{"run", "--format", "json", "--select", "reachability", "--timeout", "8", "--out", before}, &out, &errb, ex)
	if code != 0 && code != 1 {
		t.Fatalf("unexpected run(before) code=%d stderr=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()

	code = runCLI(context.Background(), []string{"run", "--format", "json", "--select", "reachability", "--timeout", "8", "--out", after}, &out, &errb, ex)
	if code != 0 && code != 1 {
		t.Fatalf("unexpected run(after) code=%d stderr=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()

	code = runCLI(context.Background(), []string{"compare", "--format", "json", before, after}, &out, &errb, ex)
	if code != 0 {
		t.Fatalf("unexpected compare code=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "before_score") || !strings.Contains(out.String(), "after_score") {
		t.Fatalf("unexpected compare output: %s", out.String())
	}
}

func TestIntegrationManExportSmoke(t *testing.T) {
	var out, errb bytes.Buffer
	dir := t.TempDir()
	code := runCLI(context.Background(), []string{"man", "--export", dir, "run"}, &out, &errb, execx.RealExecutor{})
	if code != 0 {
		t.Fatalf("unexpected man code=%d stderr=%s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "netcheck-run.md")); err != nil {
		t.Fatalf("expected exported markdown file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "netcheck-run.1")); err != nil {
		t.Fatalf("expected exported roff file: %v", err)
	}
}
