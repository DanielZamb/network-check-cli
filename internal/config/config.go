package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Targets struct {
		Ping      []string `json:"ping"`
		DNSDomain []string `json:"dns_domains"`
		Resolvers []string `json:"resolvers"`
		HTTPURLs  []string `json:"http_urls"`
	} `json:"targets"`
	Bandwidth struct {
		Speedtest struct {
			Enabled  bool   `json:"enabled"`
			ServerID string `json:"server_id"`
		} `json:"speedtest"`
		Iperf struct {
			Enabled         bool   `json:"enabled"`
			Target          string `json:"target"`
			ParallelStreams int    `json:"parallel_streams"`
			DurationSec     int    `json:"duration_sec"`
		} `json:"iperf"`
	} `json:"bandwidth"`
	ExpectedPlan struct {
		DownloadMbps float64 `json:"download_mbps"`
		UploadMbps   float64 `json:"upload_mbps"`
	} `json:"expected_plan"`
	Thresholds Thresholds `json:"thresholds"`
	Soak       struct {
		IntervalSec      int  `json:"interval_sec"`
		DurationSec      int  `json:"duration_sec"`
		EmitFinalSummary bool `json:"emit_final_summary"`
	} `json:"soak"`
	PerCheckTimeoutSec int `json:"per_check_timeout_sec"`
}

type Thresholds struct {
	LossPassMax              float64 `json:"loss_pass_max"`
	LossWarnMax              float64 `json:"loss_warn_max"`
	RTTP95PassMaxMs          float64 `json:"rtt_p95_pass_max_ms"`
	RTTP95WarnMaxMs          float64 `json:"rtt_p95_warn_max_ms"`
	JitterPassMaxMs          float64 `json:"jitter_pass_max_ms"`
	JitterWarnMaxMs          float64 `json:"jitter_warn_max_ms"`
	DNSPassMaxMs             float64 `json:"dns_pass_max_ms"`
	DNSWarnMaxMs             float64 `json:"dns_warn_max_ms"`
	HTTPPassMaxMs            float64 `json:"http_pass_max_ms"`
	HTTPWarnMaxMs            float64 `json:"http_warn_max_ms"`
	LoadedLatencyPassDeltaMs float64 `json:"loaded_latency_pass_delta_ms"`
	LoadedLatencyWarnDeltaMs float64 `json:"loaded_latency_warn_delta_ms"`
	ThroughputPassPct        float64 `json:"throughput_pass_pct"`
	ThroughputWarnPct        float64 `json:"throughput_warn_pct"`
}

func Defaults() Config {
	var c Config
	c.Targets.Ping = []string{"1.1.1.1", "8.8.8.8"}
	c.Targets.DNSDomain = []string{"google.com"}
	c.Targets.Resolvers = []string{}
	c.Targets.HTTPURLs = []string{"https://example.com"}
	c.Bandwidth.Speedtest.Enabled = true
	c.Bandwidth.Iperf.Enabled = true
	c.Bandwidth.Iperf.ParallelStreams = 4
	c.Bandwidth.Iperf.DurationSec = 30
	c.Soak.IntervalSec = 5
	c.Soak.DurationSec = 0
	c.Soak.EmitFinalSummary = true
	c.PerCheckTimeoutSec = 20
	c.Thresholds = Thresholds{
		LossPassMax: 0.5, LossWarnMax: 2,
		RTTP95PassMaxMs: 40, RTTP95WarnMaxMs: 80,
		JitterPassMaxMs: 10, JitterWarnMaxMs: 25,
		DNSPassMaxMs: 50, DNSWarnMaxMs: 120,
		HTTPPassMaxMs: 800, HTTPWarnMaxMs: 2000,
		LoadedLatencyPassDeltaMs: 30, LoadedLatencyWarnDeltaMs: 80,
		ThroughputPassPct: 80, ThroughputWarnPct: 60,
	}
	return c
}

func (c Config) AsMap() map[string]any {
	b, _ := json.Marshal(c)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	var merged map[string]any
	switch ext {
	case ".json":
		if err := json.Unmarshal(b, &merged); err != nil {
			return cfg, fmt.Errorf("invalid json config: %w", err)
		}
	default:
		merged, err = parseYAMLSubset(string(b))
		if err != nil {
			return cfg, err
		}
	}
	if err := applyMap(&cfg, merged); err != nil {
		return cfg, err
	}
	return cfg, validate(cfg)
}

func validate(c Config) error {
	if len(c.Targets.Ping) == 0 {
		return errors.New("targets.ping must not be empty")
	}
	if c.Bandwidth.Iperf.Enabled && c.Bandwidth.Iperf.Target != "" {
		if strings.HasPrefix(c.Bandwidth.Iperf.Target, "127.0.0.1") || strings.HasPrefix(c.Bandwidth.Iperf.Target, "localhost") {
			return errors.New("bandwidth.iperf.target must be remote; localhost is not allowed")
		}
	}
	if c.Thresholds.ThroughputWarnPct > c.Thresholds.ThroughputPassPct {
		return errors.New("throughput_warn_pct cannot exceed throughput_pass_pct")
	}
	return nil
}

func applyMap(cfg *Config, m map[string]any) error {
	base, _ := json.Marshal(cfg)
	current := map[string]any{}
	_ = json.Unmarshal(base, &current)
	deepMerge(current, m)
	merged, _ := json.Marshal(current)
	return json.Unmarshal(merged, cfg)
}

func deepMerge(dst map[string]any, src map[string]any) {
	for k, v := range src {
		if vm, ok := v.(map[string]any); ok {
			dm, ok := dst[k].(map[string]any)
			if !ok {
				dm = map[string]any{}
				dst[k] = dm
			}
			deepMerge(dm, vm)
			continue
		}
		dst[k] = v
	}
}

// parseYAMLSubset parses a minimal YAML subset: nested maps via indentation and lists with "- " entries.
func parseYAMLSubset(src string) (map[string]any, error) {
	root := map[string]any{}
	type frame struct {
		indent int
		key    string
		obj    map[string]any
	}
	stack := []frame{{indent: -1, obj: root}}
	sc := bufio.NewScanner(strings.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		for len(stack) > 1 && indent <= stack[len(stack)-1].indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1].obj
		if strings.HasPrefix(trim, "- ") {
			return nil, fmt.Errorf("invalid yaml: list item without key context: %s", line)
		}
		parts := strings.SplitN(trim, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid yaml line: %s", line)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.TrimSpace(stripInlineComment(val))
		if val == "" {
			next := map[string]any{}
			parent[key] = next
			stack = append(stack, frame{indent: indent, key: key, obj: next})
			continue
		}
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			items := strings.Split(strings.TrimSuffix(strings.TrimPrefix(val, "["), "]"), ",")
			arr := make([]any, 0, len(items))
			for _, it := range items {
				t := stripInlineComment(strings.TrimSpace(it))
				t = strings.Trim(strings.TrimSpace(t), `"'`)
				if t == "" {
					continue
				}
				arr = append(arr, t)
			}
			parent[key] = arr
			continue
		}
		if v, err := parseScalar(val); err == nil {
			parent[key] = v
		} else {
			parent[key] = strings.Trim(val, `"'`)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return root, nil
}

func parseScalar(v string) (any, error) {
	s := strings.Trim(v, `"'`)
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	if s != "" {
		return s, nil
	}
	return nil, errors.New("empty scalar")
}

func stripInlineComment(s string) string {
	var b strings.Builder
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return strings.TrimSpace(b.String())
			}
		}
		b.WriteByte(ch)
	}
	return strings.TrimSpace(b.String())
}
