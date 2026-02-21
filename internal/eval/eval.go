package eval

import "netcheck/internal/model"

type Thresholds struct {
	Pass float64
	Warn float64
}

func UpperIsBetter(value, passMin, warnMin float64) model.Status {
	if value >= passMin {
		return model.StatusPass
	}
	if value >= warnMin {
		return model.StatusWarn
	}
	return model.StatusFail
}

func LowerIsBetter(value, passMax, warnMax float64) model.Status {
	if value < passMax {
		return model.StatusPass
	}
	if value <= warnMax {
		return model.StatusWarn
	}
	return model.StatusFail
}

func Score(checks []model.CheckResult) int {
	if len(checks) == 0 {
		return 0
	}
	weights := map[string]float64{
		"reliability": 35,
		"latency":     25,
		"dns":         10,
		"http":        10,
		"throughput":  20,
	}
	catScores := map[string][]float64{}
	for _, c := range checks {
		cat := categoryForGroup(c.Group)
		if cat == "" {
			continue
		}
		catScores[cat] = append(catScores[cat], statusToScore(c.Status))
	}
	var weightedTotal float64
	var totalWeight float64
	for cat, scores := range catScores {
		w := weights[cat]
		if w == 0 || len(scores) == 0 {
			continue
		}
		var sum float64
		for _, sc := range scores {
			sum += sc
		}
		weightedTotal += (sum / float64(len(scores))) * w
		totalWeight += w
	}
	if totalWeight == 0 {
		return 0
	}
	s := int((weightedTotal / totalWeight) * 100)
	if s < 0 {
		return 0
	}
	if s > 100 {
		return 100
	}
	return s
}

func statusToScore(s model.Status) float64 {
	switch s {
	case model.StatusPass:
		return 1
	case model.StatusWarn:
		return 0.6
	case model.StatusSkip:
		return 0.5
	default:
		return 0
	}
}

func categoryForGroup(group string) string {
	switch group {
	case "local", "reachability", "path":
		return "reliability"
	case "bufferbloat":
		return "latency"
	case "dns":
		return "dns"
	case "http":
		return "http"
	case "bandwidth":
		return "throughput"
	default:
		return ""
	}
}
