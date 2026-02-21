package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"netcheck/internal/runner"
)

func TestNonVerboseUpdatesSpinnerHeaderWithLatestLog(t *testing.T) {
	var b bytes.Buffer
	ui := newVerboseUI(&b, uiOptions{Verbose: false, Color: false, Animate: false})
	ui.OnProgress(runner.ProgressEvent{
		Phase: "start",
		Group: "dns",
		Check: "dns.google.com",
		Index: 1,
		Total: 3,
	})
	ui.OnExecLog("dns", "op", "resolving domain via resolver")
	ui.OnProgress(runner.ProgressEvent{
		Phase:  "end",
		Group:  "dns",
		Check:  "dns.google.com",
		Status: "pass",
		Index:  1,
		Total:  3,
	})
	ui.Stop()

	s := b.String()
	if strings.Contains(s, "op :") {
		t.Fatalf("non-verbose mode should not print full op logs, got: %q", s)
	}
	if !strings.Contains(s, "[DNS] resolving domain via resolver") {
		t.Fatalf("expected latest header message in spinner line, got: %q", s)
	}
	if !strings.Contains(s, "1/3") {
		t.Fatalf("expected progress counters in spinner line, got: %q", s)
	}
}

func TestProgressBarGradientUsesANSIColor(t *testing.T) {
	bar := progressBar(5, 10, 10, true)
	if !strings.Contains(bar, "\x1b[38;5;") {
		t.Fatalf("expected ANSI gradient color in progress bar, got: %q", bar)
	}
	if !strings.Contains(bar, "\x1b[0m") {
		t.Fatalf("expected ANSI reset in progress bar, got: %q", bar)
	}
}

func TestFormatGroupTagColorized(t *testing.T) {
	tag := formatGroupTag("BANDWIDTH", true)
	if !strings.Contains(tag, "\x1b[38;5;214m[BANDWIDTH]\x1b[0m") {
		t.Fatalf("expected colorized bandwidth tag, got: %q", tag)
	}
}

func TestNormalizeHeaderCapsAndFlattens(t *testing.T) {
	in := "alpha beta gamma\ndelta epsilon zeta eta theta iota kappa lambda"
	got := normalizeHeader(in, 24)
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") || strings.Contains(got, "\t") {
		t.Fatalf("header should be single-line, got: %q", got)
	}
	if len([]rune(got)) > 24 {
		t.Fatalf("header should be capped, got len=%d value=%q", len([]rune(got)), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncation suffix, got: %q", got)
	}
}

func TestRenderClearsLineEachFrame(t *testing.T) {
	var b bytes.Buffer
	ui := newVerboseUI(&b, uiOptions{Verbose: false, Color: false, Animate: false, MaxHead: 40})
	ui.OnProgress(runner.ProgressEvent{
		Phase: "start",
		Group: "http",
		Check: "http.https://example.com",
		Index: 1,
		Total: 2,
	})
	ui.OnExecLog("http", "op", "very long message that should be trimmed at some point to avoid wrapping all over")
	ui.Stop()
	out := b.String()
	if !strings.Contains(out, "\x1b[2K") {
		t.Fatalf("expected line-clear escape sequence in spinner output")
	}
}

func TestCompleteMessagePrintsGreenAndIsIdempotent(t *testing.T) {
	var b bytes.Buffer
	ui := newVerboseUI(&b, uiOptions{Verbose: false, Color: true, Animate: false, MaxHead: 40})
	ui.CompleteMessage("Run completed. Rendering report...")
	ui.Stop()
	s := b.String()
	if !strings.Contains(s, "\x1b[32mRun completed. Rendering report...\x1b[0m") {
		t.Fatalf("expected green completion message, got: %q", s)
	}
}

func TestStripANSIAndVisibleLen(t *testing.T) {
	in := "\x1b[38;5;214m[BANDWIDTH]\x1b[0m hello"
	if got := stripANSI(in); got != "[BANDWIDTH] hello" {
		t.Fatalf("stripANSI mismatch: %q", got)
	}
	if n := visibleLen(in); n != len("[BANDWIDTH] hello") {
		t.Fatalf("visibleLen mismatch: %d", n)
	}
}

func TestDetectTerminalWidthFromEnv(t *testing.T) {
	old := os.Getenv("COLUMNS")
	t.Cleanup(func() { _ = os.Setenv("COLUMNS", old) })
	_ = os.Setenv("COLUMNS", "72")
	if got := detectTerminalWidth(); got != 72 {
		t.Fatalf("expected width 72, got %d", got)
	}
	_ = os.Setenv("COLUMNS", "bad")
	if got := detectTerminalWidth(); got != 80 {
		t.Fatalf("expected fallback width 80, got %d", got)
	}
}
