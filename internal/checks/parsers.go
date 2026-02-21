package checks

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	packetLossRe = regexp.MustCompile(`([0-9.]+)% packet loss`)
	rttRe        = regexp.MustCompile(`min/avg/max/(?:stddev|mdev) = ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+) ms`)
	digMsRe      = regexp.MustCompile(`Query time: ([0-9]+) msec`)
	pingTimeRe   = regexp.MustCompile(`time=([0-9.]+)\s*ms`)
	mtrLineRe    = regexp.MustCompile(`^\s*\d+\.\|--\s+\S+\s+([0-9.]+)%`)
)

func parsePing(output string) (loss, avg, jitter, p95 float64) {
	if m := packetLossRe.FindStringSubmatch(output); len(m) == 2 {
		loss, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := rttRe.FindStringSubmatch(output); len(m) == 5 {
		avg, _ = strconv.ParseFloat(m[2], 64)
		jitter, _ = strconv.ParseFloat(m[4], 64)
	}
	samples := parsePingTimeSamples(output)
	if len(samples) > 0 {
		p95 = percentile(samples, 95)
	} else {
		p95 = avg
	}
	return
}

func parsePingTimeSamples(output string) []float64 {
	matches := pingTimeRe.FindAllStringSubmatch(output, -1)
	out := make([]float64, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 100 {
		return cp[len(cp)-1]
	}
	rank := int(math.Ceil((p / 100) * float64(len(cp))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(cp) {
		rank = len(cp)
	}
	return cp[rank-1]
}

func parseMTRSummary(output string) (hopCount int, nearDestLossPct float64) {
	lines := strings.Split(output, "\n")
	losses := make([]float64, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := mtrLineRe.FindStringSubmatch(line)
		if len(m) == 2 {
			l, err := strconv.ParseFloat(m[1], 64)
			if err == nil {
				losses = append(losses, l)
			}
		}
	}
	hopCount = len(losses)
	if hopCount > 0 {
		nearDestLossPct = losses[hopCount-1]
	}
	return
}

func parseTracerouteSummary(output string) (hopCount int, timeoutHops int) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "traceroute ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			if _, err := strconv.Atoi(fields[0]); err == nil {
				hopCount++
				if strings.Count(line, "*") >= 3 {
					timeoutHops++
				}
			}
		}
	}
	return
}

func parseDigMS(output string) float64 {
	if m := digMsRe.FindStringSubmatch(output); len(m) == 2 {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}
	return 0
}

func parseCurlTimings(output string) map[string]float64 {
	parts := strings.Fields(strings.TrimSpace(output))
	res := map[string]float64{}
	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			continue
		}
		v, err := strconv.ParseFloat(kv[1], 64)
		if err != nil {
			continue
		}
		res[kv[0]] = v * 1000
	}
	return res
}
