package model

import "testing"

func TestSummaryAdd(t *testing.T) {
	var s Summary
	s.Add(StatusPass)
	s.Add(StatusWarn)
	s.Add(StatusFail)
	s.Add(StatusSkip)
	if s.Total != 4 || s.Pass != 1 || s.Warn != 1 || s.Fail != 1 || s.Skip != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
}
