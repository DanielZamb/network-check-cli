package exitcode

import (
	"netcheck/internal/model"
	"testing"
)

func TestFromSummary(t *testing.T) {
	if c := FromSummary(model.Summary{Fail: 1}, false); c != ChecksFailed {
		t.Fatalf("expected fail code")
	}
	if c := FromSummary(model.Summary{Warn: 1}, true); c != ChecksFailed {
		t.Fatalf("expected strict warn fail code")
	}
	if c := FromSummary(model.Summary{Pass: 1}, false); c != OK {
		t.Fatalf("expected ok")
	}
}
