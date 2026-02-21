package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
	Duration time.Duration
}

type Executor interface {
	Run(ctx context.Context, name string, args ...string) Result
	LookPath(file string) (string, error)
}

type RealExecutor struct{}

type LogFunc func(group, op, msg string)

type loggerKey struct{}
type groupKey struct{}
type checkKey struct{}

func WithLogFunc(ctx context.Context, fn LogFunc) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, fn)
}

func WithCheckMetadata(ctx context.Context, group, check string) context.Context {
	ctx = context.WithValue(ctx, groupKey{}, group)
	ctx = context.WithValue(ctx, checkKey{}, check)
	return ctx
}

func logf(ctx context.Context, op, format string, args ...any) {
	fn, ok := ctx.Value(loggerKey{}).(LogFunc)
	if !ok || fn == nil {
		return
	}
	group, _ := ctx.Value(groupKey{}).(string)
	if group == "" {
		group = "SYSTEM"
	}
	msg := fmt.Sprintf(format, args...)
	fn(group, op, msg)
}

func describeCommand(name string, args []string) string {
	switch name {
	case "netstat":
		return "discovering routing/gateway information"
	case "ifconfig":
		return "collecting local interface information"
	case "ping":
		if len(args) > 0 {
			return "measuring latency/loss to target"
		}
		return "measuring latency/loss"
	case "dig":
		return "performing DNS lookup"
	case "curl":
		return "measuring HTTP/TLS timings"
	case "mtr":
		return "collecting path quality and hop loss"
	case "traceroute":
		return "collecting route hop path"
	case "speedtest-cli":
		return "running internet throughput test"
	case "iperf3":
		return "running controlled throughput test"
	case "openssl":
		return "reading TLS handshake metadata"
	default:
		return "running command"
	}
}

func (RealExecutor) Run(ctx context.Context, name string, args ...string) Result {
	start := time.Now()
	logf(ctx, "op", "%s; calling exec with flags: %s %s", describeCommand(name, args), name, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	outStr := out.String()
	errStr := errb.String()
	for _, line := range strings.Split(strings.TrimSpace(outStr), "\n") {
		if strings.TrimSpace(line) != "" {
			logf(ctx, "op", "logs: %s", line)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(errStr), "\n") {
		if strings.TrimSpace(line) != "" {
			logf(ctx, "op", "stderr: %s", line)
		}
	}
	if err != nil {
		logf(ctx, "op", "exec error: %v", err)
	}
	return Result{Stdout: outStr, Stderr: errStr, ExitCode: code, Err: err, Duration: time.Since(start)}
}

func (RealExecutor) LookPath(file string) (string, error) { return exec.LookPath(file) }

type FakeExecutor struct {
	Outputs map[string]Result
	Paths   map[string]bool
	Delays  map[string]time.Duration
	Calls   []string
}

func (f *FakeExecutor) key(name string, args ...string) string {
	k := name
	for _, a := range args {
		k += " " + a
	}
	return k
}

func (f *FakeExecutor) Run(ctx context.Context, name string, args ...string) Result {
	return f.run(ctx, name, args...)
}

func (f *FakeExecutor) run(ctx context.Context, name string, args ...string) Result {
	k := f.key(name, args...)
	f.Calls = append(f.Calls, k)
	logf(ctx, "op", "%s; calling exec with flags: %s", describeCommand(name, args), k)
	if d, ok := f.Delays[k]; ok && d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			logf(ctx, "op", "exec timeout: %v", ctx.Err())
			return Result{ExitCode: -1, Err: ctx.Err()}
		}
	}
	if r, ok := f.Outputs[k]; ok {
		for _, line := range strings.Split(strings.TrimSpace(r.Stdout), "\n") {
			if strings.TrimSpace(line) != "" {
				logf(ctx, "op", "logs: %s", line)
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(r.Stderr), "\n") {
			if strings.TrimSpace(line) != "" {
				logf(ctx, "op", "stderr: %s", line)
			}
		}
		if r.Err != nil {
			logf(ctx, "op", "exec error: %v", r.Err)
		}
		return r
	}
	logf(ctx, "op", "exec error: no fake output configured")
	return Result{ExitCode: 127, Err: errors.New("no fake output configured")}
}

func (f *FakeExecutor) LookPath(file string) (string, error) {
	if f.Paths == nil {
		return "", errors.New("not found")
	}
	if f.Paths[file] {
		return "/usr/bin/" + file, nil
	}
	return "", errors.New("not found")
}

func HasTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")
}
