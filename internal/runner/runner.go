package runner

import (
	"context"
	"netcheck/internal/checks"
	"netcheck/internal/config"
	"netcheck/internal/eval"
	"netcheck/internal/execx"
	"netcheck/internal/model"
	"netcheck/internal/schema"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

type RunResult struct {
	Report model.Report
}

func BuildChecks(cfg config.Config) []checks.Check {
	all := []checks.Check{checks.LocalCheck{}, checks.SpeedtestCheck{}, checks.IperfCheck{}}
	for _, p := range cfg.Targets.Ping {
		all = append(all, checks.ReachabilityCheck{Target: p})
	}
	for _, d := range cfg.Targets.DNSDomain {
		all = append(all, checks.DNSCheck{Domain: d})
		for _, r := range cfg.Targets.Resolvers {
			all = append(all, checks.DNSCheck{Domain: d, Resolver: r})
		}
	}
	for _, u := range cfg.Targets.HTTPURLs {
		all = append(all, checks.HTTPCheck{URL: u})
	}
	if len(cfg.Targets.Ping) > 0 {
		all = append(all, checks.PathCheck{Target: cfg.Targets.Ping[0]})
		all = append(all, checks.BufferbloatCheck{Target: cfg.Targets.Ping[0]})
	}
	return all
}

func filterChecks(all []checks.Check, selectGroups, skipGroups []string) []checks.Check {
	if len(selectGroups) == 0 && len(skipGroups) == 0 {
		return all
	}
	sel := map[string]bool{}
	skp := map[string]bool{}
	for _, g := range selectGroups {
		sel[strings.TrimSpace(g)] = true
	}
	for _, g := range skipGroups {
		skp[strings.TrimSpace(g)] = true
	}
	out := make([]checks.Check, 0, len(all))
	for _, c := range all {
		if len(sel) > 0 && !sel[c.Group()] {
			continue
		}
		if skp[c.Group()] {
			continue
		}
		out = append(out, c)
	}
	return out
}

func SelectedChecks(cfg config.Config, opts model.RunOptions) []checks.Check {
	return filterChecks(BuildChecks(cfg), opts.Select, opts.Skip)
}

func RunOnce(ctx context.Context, ex execx.Executor, cfg config.Config, opts model.RunOptions, version, commit string) (RunResult, error) {
	all := SelectedChecks(cfg, opts)
	res := make([]model.CheckResult, 0, len(all))
	summary := model.Summary{}
	for i, c := range all {
		reportProgress(ctx, ProgressEvent{Phase: "start", Check: c.ID(), Group: c.Group(), Index: i + 1, Total: len(all)})
		cctx := execx.WithCheckMetadata(ctx, strings.ToUpper(c.Group()), c.ID())
		cr := c.Run(cctx, ex, cfg, cfg.PerCheckTimeoutSec)
		reportProgress(ctx, ProgressEvent{Phase: "end", Check: c.ID(), Group: c.Group(), Index: i + 1, Total: len(all), Status: cr.Status})
		summary.Add(cr.Status)
		res = append(res, cr)
		if opts.FailFast && cr.Status == model.StatusFail {
			break
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].ID < res[j].ID })
	host, _ := os.Hostname()
	report := model.Report{
		SchemaVersion: schema.Version,
		Timestamp:     time.Now().UTC(),
		Host:          host,
		OS:            runtime.GOOS,
		Version:       version,
		GitCommit:     commit,
		RunID:         opts.RunID,
		Labels:        opts.Labels,
		Config:        cfg.AsMap(),
		Checks:        res,
		Summary:       summary,
		Score:         eval.Score(res),
	}
	return RunResult{Report: report}, nil
}
