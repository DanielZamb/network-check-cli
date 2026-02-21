package eval

import (
	"netcheck/internal/model"
	"testing"
)

func TestScore(t *testing.T) {
	checks := []model.CheckResult{
		{Group: "reachability", Status: model.StatusPass},
		{Group: "dns", Status: model.StatusWarn},
		{Group: "http", Status: model.StatusFail},
	}
	s := Score(checks)
	if s <= 0 || s >= 100 {
		t.Fatalf("unexpected score %d", s)
	}
}

func TestScoreWeightedCategories(t *testing.T) {
	checks := []model.CheckResult{
		{Group: "reachability", Status: model.StatusPass},
		{Group: "dns", Status: model.StatusFail},
		{Group: "http", Status: model.StatusPass},
		{Group: "bandwidth", Status: model.StatusWarn},
		{Group: "bufferbloat", Status: model.StatusPass},
	}
	s := Score(checks)
	if s <= 0 || s >= 100 {
		t.Fatalf("unexpected weighted score %d", s)
	}
}

func TestUpperIsBetter(t *testing.T) {
	if got := UpperIsBetter(90, 80, 60); got != model.StatusPass {
		t.Fatalf("expected pass, got %s", got)
	}
	if got := UpperIsBetter(70, 80, 60); got != model.StatusWarn {
		t.Fatalf("expected warn, got %s", got)
	}
	if got := UpperIsBetter(50, 80, 60); got != model.StatusFail {
		t.Fatalf("expected fail, got %s", got)
	}
}
