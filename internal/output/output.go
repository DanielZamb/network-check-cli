package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"netcheck/internal/model"
	"sort"
	"strings"
	"text/tabwriter"
)

type TableOptions struct {
	Color bool
}

func WriteJSON(w io.Writer, report model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func JSONBytes(report model.Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func WriteTable(w io.Writer, report model.Report) error {
	return WriteTableWithOptions(w, report, TableOptions{})
}

func WriteTableWithOptions(w io.Writer, report model.Report, opts TableOptions) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tGROUP\tSTATUS\tTARGET")
	_, _ = fmt.Fprintln(tw, "--\t-----\t------\t------")
	checks := make([]model.CheckResult, len(report.Checks))
	copy(checks, report.Checks)
	sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	for _, c := range checks {
		status := string(c.Status)
		group := c.Group
		if opts.Color {
			status = colorStatus(c.Status)
			group = colorGroup(group)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.ID, group, status, c.Target)
	}
	_, _ = fmt.Fprintf(tw, "\nSummary\tpass=%d warn=%d fail=%d skip=%d total=%d\t\t\n", report.Summary.Pass, report.Summary.Warn, report.Summary.Fail, report.Summary.Skip, report.Summary.Total)
	_, _ = fmt.Fprintf(tw, "Score\t%d\t\t\n", report.Score)
	groupRows := buildGroupRows(report)
	if len(groupRows) > 0 {
		_, _ = fmt.Fprintln(tw, "\nGroup Summary\t\t\t")
		_, _ = fmt.Fprintln(tw, "GROUP\tSCORE\tMEASURED\tEXPECTED")
		_, _ = fmt.Fprintln(tw, "-----\t-----\t--------\t--------")
		for _, r := range groupRows {
			group := r.Group
			if opts.Color {
				group = colorGroup(group)
			}
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", group, r.Score, r.Measured, r.Expected)
		}
	}
	return tw.Flush()
}

func TableString(report model.Report) (string, error) {
	var b bytes.Buffer
	if err := WriteTable(&b, report); err != nil {
		return "", err
	}
	return b.String(), nil
}

func colorStatus(s model.Status) string {
	switch s {
	case model.StatusPass:
		return "\x1b[32m" + string(s) + "\x1b[0m"
	case model.StatusWarn:
		return "\x1b[33m" + string(s) + "\x1b[0m"
	case model.StatusFail:
		return "\x1b[31m" + string(s) + "\x1b[0m"
	default:
		return string(s)
	}
}

func colorGroup(group string) string {
	code := "250"
	switch strings.ToLower(group) {
	case "local":
		code = "39"
	case "bandwidth":
		code = "214"
	case "dns":
		code = "33"
	case "http":
		code = "75"
	case "path":
		code = "208"
	case "reachability":
		code = "45"
	case "bufferbloat":
		code = "171"
	}
	return fmt.Sprintf("\x1b[38;5;%sm%s\x1b[0m", code, group)
}

type groupRow struct {
	Group    string
	Score    int
	Measured string
	Expected string
}

func buildGroupRows(report model.Report) []groupRow {
	byGroup := map[string][]model.CheckResult{}
	for _, c := range report.Checks {
		byGroup[c.Group] = append(byGroup[c.Group], c)
	}
	groups := make([]string, 0, len(byGroup))
	for g := range byGroup {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	out := make([]groupRow, 0, len(groups))
	for _, g := range groups {
		cs := byGroup[g]
		out = append(out, groupRow{
			Group:    g,
			Score:    computeGroupScore(cs),
			Measured: measuredForGroup(g, cs),
			Expected: expectedForGroup(g, report.Config),
		})
	}
	return out
}

func computeGroupScore(cs []model.CheckResult) int {
	if len(cs) == 0 {
		return 0
	}
	var sum float64
	for _, c := range cs {
		switch c.Status {
		case model.StatusPass:
			sum += 1
		case model.StatusWarn:
			sum += 0.6
		case model.StatusSkip:
			sum += 0.5
		}
	}
	return int(math.Round(sum / float64(len(cs)) * 100))
}

func measuredForGroup(group string, cs []model.CheckResult) string {
	switch group {
	case "bandwidth":
		dl := metricAvg(cs, "download_mbps")
		ul := metricAvg(cs, "upload_mbps")
		if !math.IsNaN(dl) && !math.IsNaN(ul) {
			return fmt.Sprintf("dl=%.1fMbps ul=%.1fMbps", dl, ul)
		}
		if !math.IsNaN(dl) {
			return fmt.Sprintf("dl=%.1fMbps", dl)
		}
	case "dns":
		q := metricAvg(cs, "query_ms")
		if !math.IsNaN(q) {
			return fmt.Sprintf("avg_query=%.1fms", q)
		}
	case "http":
		t := metricAvg(cs, "total_ms")
		if !math.IsNaN(t) {
			return fmt.Sprintf("avg_total=%.1fms", t)
		}
	case "reachability":
		p95 := metricAvg(cs, "rtt_p95_ms")
		loss := metricAvg(cs, "loss_pct")
		if !math.IsNaN(p95) && !math.IsNaN(loss) {
			return fmt.Sprintf("p95=%.1fms loss=%.2f%%", p95, loss)
		}
	case "local":
		loss := metricAvg(cs, "loss_pct")
		if !math.IsNaN(loss) {
			return fmt.Sprintf("loss=%.2f%%", loss)
		}
	case "path":
		l := metricAvg(cs, "near_dest_loss_pct")
		h := metricAvg(cs, "hop_count")
		if !math.IsNaN(l) && !math.IsNaN(h) {
			return fmt.Sprintf("near_loss=%.2f%% hops=%.0f", l, h)
		}
		th := metricAvg(cs, "timeout_hops")
		if !math.IsNaN(h) && !math.IsNaN(th) {
			return fmt.Sprintf("hops=%.0f timeout_hops=%.0f", h, th)
		}
	case "bufferbloat":
		d := metricAvg(cs, "delta_ms")
		if !math.IsNaN(d) {
			return fmt.Sprintf("delta=%.1fms", d)
		}
	}
	return "-"
}

func expectedForGroup(group string, cfg map[string]any) string {
	switch group {
	case "bandwidth":
		dl := cfgFloat(cfg, "expected_plan", "download_mbps")
		ul := cfgFloat(cfg, "expected_plan", "upload_mbps")
		passPct := cfgFloat(cfg, "thresholds", "throughput_pass_pct")
		if dl > 0 || ul > 0 {
			parts := []string{}
			if dl > 0 {
				parts = append(parts, fmt.Sprintf("dl>=%.1fMbps", dl))
			}
			if ul > 0 {
				parts = append(parts, fmt.Sprintf("ul>=%.1fMbps", ul))
			}
			if passPct > 0 {
				parts = append(parts, fmt.Sprintf("pass@%.0f%%", passPct))
			}
			return strings.Join(parts, " ")
		}
		return "no plan target"
	case "dns":
		return fmt.Sprintf("pass<%.0fms warn<=%.0fms", cfgFloat(cfg, "thresholds", "dns_pass_max_ms"), cfgFloat(cfg, "thresholds", "dns_warn_max_ms"))
	case "http":
		return fmt.Sprintf("pass<%.0fms warn<=%.0fms", cfgFloat(cfg, "thresholds", "http_pass_max_ms"), cfgFloat(cfg, "thresholds", "http_warn_max_ms"))
	case "reachability":
		return fmt.Sprintf("loss<%.1f%% p95<%.0fms jitter<%.0fms", cfgFloat(cfg, "thresholds", "loss_pass_max"), cfgFloat(cfg, "thresholds", "rtt_p95_pass_max_ms"), cfgFloat(cfg, "thresholds", "jitter_pass_max_ms"))
	case "local":
		return fmt.Sprintf("loss<%.1f%%", cfgFloat(cfg, "thresholds", "loss_pass_max"))
	case "path":
		return fmt.Sprintf("near_loss<%.1f%%", cfgFloat(cfg, "thresholds", "loss_pass_max"))
	case "bufferbloat":
		return fmt.Sprintf("delta<%.0fms", cfgFloat(cfg, "thresholds", "loaded_latency_pass_delta_ms"))
	default:
		return "-"
	}
}

func metricAvg(cs []model.CheckResult, key string) float64 {
	var sum float64
	var n int
	for _, c := range cs {
		if c.Metrics == nil {
			continue
		}
		if v, ok := c.Metrics[key]; ok {
			switch x := v.(type) {
			case float64:
				sum += x
				n++
			case float32:
				sum += float64(x)
				n++
			case int:
				sum += float64(x)
				n++
			case int64:
				sum += float64(x)
				n++
			}
		}
	}
	if n == 0 {
		return math.NaN()
	}
	return sum / float64(n)
}

func cfgFloat(cfg map[string]any, path ...string) float64 {
	var cur any = cfg
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return 0
		}
		cur, ok = m[p]
		if !ok {
			return 0
		}
	}
	switch v := cur.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}
