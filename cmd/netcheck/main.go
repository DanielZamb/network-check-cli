package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"netcheck/internal/compare"
	"netcheck/internal/config"
	"netcheck/internal/docs"
	"netcheck/internal/events"
	"netcheck/internal/execx"
	"netcheck/internal/exitcode"
	"netcheck/internal/model"
	"netcheck/internal/output"
	"netcheck/internal/runner"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	version = "dev"
	commit  = ""
)

func main() {
	os.Exit(runCLI(context.Background(), os.Args[1:], os.Stdout, os.Stderr, execx.RealExecutor{}))
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer, ex execx.Executor) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: netcheck <run|soak|compare|man>")
		return exitcode.ConfigError
	}
	switch args[0] {
	case "run":
		return cmdRun(ctx, args[1:], stdout, stderr, ex)
	case "soak":
		return cmdSoak(ctx, args[1:], stdout, stderr, ex)
	case "compare":
		return cmdCompare(args[1:], stdout, stderr)
	case "man":
		return cmdMan(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "unknown command:", args[0])
		return exitcode.ConfigError
	}
}

func parseCommon(fs *flag.FlagSet) *model.RunOptions {
	opts := &model.RunOptions{}
	var labels, selectGroups, skipGroups string
	fs.StringVar(&opts.Format, "format", "table", "table|json|jsonl|both")
	fs.StringVar(&opts.OutPath, "out", "", "output file")
	fs.BoolVar(&opts.Verbose, "verbose", false, "verbose output")
	fs.BoolVar(&opts.Quiet, "quiet", false, "quiet output")
	fs.BoolVar(&opts.NoColor, "no-color", false, "no color")
	fs.BoolVar(&opts.FailFast, "fail-fast", false, "stop on first fail")
	fs.IntVar(&opts.TimeoutSec, "timeout", 180, "global timeout sec")
	fs.StringVar(&opts.RunID, "id", "", "run id")
	fs.BoolVar(&opts.StrictWarn, "strict-warn", false, "warnings count as failure")
	fs.StringVar(&opts.ConfigPath, "config", "", "config path")
	fs.StringVar(&labels, "labels", "", "labels key=value,key2=value2")
	fs.StringVar(&selectGroups, "select", "", "comma-separated group filter")
	fs.StringVar(&skipGroups, "skip", "", "comma-separated group skip")
	return opts
}

func finalizeCommon(opts *model.RunOptions, fs *flag.FlagSet) {
	opts.Select = splitCSV(fs.Lookup("select").Value.String())
	opts.Skip = splitCSV(fs.Lookup("skip").Value.String())
	opts.Labels = parseLabels(fs.Lookup("labels").Value.String())
}

func cmdRun(ctx context.Context, args []string, stdout, stderr io.Writer, ex execx.Executor) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := parseCommon(fs)
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}
	finalizeCommon(opts, fs)
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		fmt.Fprintln(stderr, "config error:", err)
		return exitcode.ConfigError
	}
	effectiveTimeout := opts.TimeoutSec
	minNeeded := estimateRunTimeoutSec(cfg, *opts)
	if effectiveTimeout < minNeeded {
		if opts.Verbose && !opts.Quiet {
			fmt.Fprintf(stderr, "[RUN] op : timeout exemption applied; requested=%ds estimated_min=%ds\n", effectiveTimeout, minNeeded)
		}
		effectiveTimeout = minNeeded
	}
	rctx, cancel := context.WithTimeout(ctx, time.Duration(effectiveTimeout)*time.Second)
	defer cancel()
	var ui *verboseUI
	if !opts.Quiet {
		ui = newVerboseUI(stderr, uiOptions{
			Verbose: opts.Verbose,
			Color:   shouldColorize(stderr, opts.NoColor),
			Animate: isTerminalWriter(stderr),
			MaxHead: 120,
		})
		defer ui.Stop()
		rctx = runner.WithProgressReporter(rctx, ui.OnProgress)
		rctx = execx.WithLogFunc(rctx, ui.OnExecLog)
	}
	result, err := runner.RunOnce(rctx, ex, cfg, *opts, version, commit)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.RuntimeError
	}
	if opts.Verbose && !opts.Quiet {
		fmt.Fprintf(stderr, "\n[RUN] op : summary score=%d pass=%d warn=%d fail=%d skip=%d\n", result.Report.Score, result.Report.Summary.Pass, result.Report.Summary.Warn, result.Report.Summary.Fail, result.Report.Summary.Skip)
	}
	if ui != nil {
		ui.CompleteMessage("Run completed. Rendering report...")
	}
	if err := emitReport(result.Report, *opts, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.OutputError
	}
	return exitcode.FromSummary(result.Report.Summary, opts.StrictWarn)
}

func cmdSoak(ctx context.Context, args []string, stdout, stderr io.Writer, ex execx.Executor) int {
	fs := flag.NewFlagSet("soak", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := parseCommon(fs)
	intervalSec := fs.Int("interval", -1, "interval seconds")
	durationSec := fs.Int("duration", -1, "duration seconds; 0 means until interrupted")
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}
	finalizeCommon(opts, fs)
	if opts.Format == "table" {
		opts.Format = "jsonl"
	}
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		fmt.Fprintln(stderr, "config error:", err)
		return exitcode.ConfigError
	}
	interval := *intervalSec
	duration := *durationSec
	if interval < 0 {
		interval = cfg.Soak.IntervalSec
	}
	if duration < 0 {
		duration = cfg.Soak.DurationSec
	}
	if interval <= 0 {
		interval = 1
	}
	sctx := ctx
	if opts.TimeoutSec > 0 {
		var cancel context.CancelFunc
		sctx, cancel = context.WithTimeout(ctx, time.Duration(opts.TimeoutSec)*time.Second)
		defer cancel()
	}
	if !opts.Quiet {
		ui := newVerboseUI(stderr, uiOptions{
			Verbose: opts.Verbose,
			Color:   shouldColorize(stderr, opts.NoColor),
			Animate: isTerminalWriter(stderr),
			MaxHead: 120,
		})
		defer ui.Stop()
		sctx = runner.WithProgressReporter(sctx, ui.OnProgress)
		sctx = execx.WithLogFunc(sctx, ui.OnExecLog)
	}
	writer, closeFn, err := outWriter(opts.OutPath, stdout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.OutputError
	}
	defer closeFn()
	ew := events.NewWriter(writer)
	if opts.RunID == "" {
		opts.RunID = fmt.Sprintf("soak-%d", time.Now().Unix())
	}
	_ = ew.Emit("run_started", opts.RunID, map[string]any{"command": "soak"})
	start := time.Now()
	lastExit := 0
	for {
		if duration > 0 && time.Since(start) > time.Duration(duration)*time.Second {
			break
		}
		if err := sctx.Err(); err != nil {
			break
		}
		res, err := runner.RunOnce(sctx, ex, cfg, *opts, version, commit)
		if err != nil {
			_ = ew.Emit("run_finished", opts.RunID, map[string]any{"error": err.Error()})
			return exitcode.RuntimeError
		}
		for _, c := range res.Report.Checks {
			_ = ew.Emit("check_result", opts.RunID, map[string]any{"id": c.ID, "status": c.Status, "target": c.Target})
		}
		_ = ew.Emit("interval_summary", opts.RunID, map[string]any{"summary": res.Report.Summary, "score": res.Report.Score})
		if opts.Verbose && !opts.Quiet {
			fmt.Fprintf(stderr, "\n[SOAK] op : interval summary score=%d pass=%d warn=%d fail=%d skip=%d\n", res.Report.Score, res.Report.Summary.Pass, res.Report.Summary.Warn, res.Report.Summary.Fail, res.Report.Summary.Skip)
		}
		lastExit = exitcode.FromSummary(res.Report.Summary, opts.StrictWarn)
		if !opts.Quiet && (opts.Format == "both" || opts.Format == "table") {
			_ = output.WriteTableWithOptions(stdout, res.Report, output.TableOptions{Color: shouldColorize(stdout, opts.NoColor)})
		}
		select {
		case <-time.After(time.Duration(interval) * time.Second):
		case <-sctx.Done():
			// global timeout/cancel.
		}
	}
	if cfg.Soak.EmitFinalSummary {
		_ = ew.Emit("run_summary", opts.RunID, map[string]any{"done": true})
	}
	_ = ew.Emit("run_finished", opts.RunID, map[string]any{"duration_sec": int(time.Since(start).Seconds())})
	return lastExit
}

func cmdCompare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "table", "table|json")
	out := fs.String("out", "", "optional output file")
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintln(stderr, "usage: netcheck compare <baseline.json> <candidate.json>")
		return exitcode.ConfigError
	}
	before, err := compare.Load(rest[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.RuntimeError
	}
	after, err := compare.Load(rest[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.RuntimeError
	}
	d := compare.Build(before, after)
	if *format == "json" {
		b, _ := json.MarshalIndent(d, "", "  ")
		if *out == "" {
			_, _ = stdout.Write(append(b, '\n'))
		} else {
			if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
				fmt.Fprintln(stderr, err)
				return exitcode.OutputError
			}
		}
		return 0
	}
	path := *out
	if path == "" {
		path = filepath.Join(os.TempDir(), "netcheck-compare.txt")
	}
	if err := compare.WriteTable(path, d); err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.OutputError
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitcode.OutputError
	}
	_, _ = stdout.Write(b)
	return 0
}

func cmdMan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("man", flag.ContinueOnError)
	fs.SetOutput(stderr)
	export := fs.String("export", "", "export manuals to directory")
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}
	if *export != "" {
		if err := docs.Export(*export); err != nil {
			fmt.Fprintln(stderr, err)
			return exitcode.RuntimeError
		}
	}
	topic := ""
	if len(fs.Args()) > 0 {
		topic = fs.Args()[0]
	}
	text, err := docs.Get(topic)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprintln(stderr, "available topics:", strings.Join(docs.Topics(), ", "))
		return exitcode.ConfigError
	}
	_, _ = fmt.Fprintln(stdout, text)
	return 0
}

func emitReport(report model.Report, opts model.RunOptions, stdout io.Writer) error {
	w, closeFn, err := outWriter(opts.OutPath, stdout)
	if err != nil {
		return err
	}
	defer closeFn()
	switch opts.Format {
	case "json":
		return output.WriteJSON(w, report)
	case "jsonl":
		b, err := json.Marshal(report)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	case "table":
		return output.WriteTableWithOptions(w, report, output.TableOptions{Color: shouldColorize(stdout, opts.NoColor)})
	case "both":
		if err := output.WriteTableWithOptions(w, report, output.TableOptions{Color: shouldColorize(stdout, opts.NoColor)}); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w)
		return output.WriteJSON(w, report)
	default:
		return errors.New("invalid format")
	}
}

func outWriter(path string, fallback io.Writer) (io.Writer, func(), error) {
	if path == "" {
		return fallback, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}

func shouldColorize(w io.Writer, noColor bool) bool {
	if noColor {
		return false
	}
	return isTerminalWriter(w)
}

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseLabels(raw string) map[string]string {
	out := map[string]string{}
	for _, pair := range splitCSV(raw) {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func estimateRunTimeoutSec(cfg config.Config, opts model.RunOptions) int {
	checks := runner.SelectedChecks(cfg, opts)
	total := 0
	for _, c := range checks {
		switch c.Group() {
		case "local":
			total += 14
		case "reachability":
			total += 12
		case "dns":
			total += 3
		case "http":
			total += 8
		case "path":
			total += 12
		case "bufferbloat":
			if cfg.Bandwidth.Iperf.Enabled && cfg.Bandwidth.Iperf.Target != "" {
				total += 30
			} else if cfg.Bandwidth.Speedtest.Enabled {
				total += 55
			} else {
				total += 25
			}
		case "bandwidth":
			id := c.ID()
			if strings.Contains(id, "speedtest") {
				total += 50
			}
			if strings.Contains(id, "iperf") {
				d := cfg.Bandwidth.Iperf.DurationSec
				if d <= 0 {
					d = 30
				}
				total += d + 15
			}
		default:
			total += 5
		}
	}
	if total < 30 {
		total = 30
	}
	return total
}
