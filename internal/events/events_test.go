package events

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmit(t *testing.T) {
	var b bytes.Buffer
	w := NewWriter(&b)
	w.SetNow(func() time.Time { return time.Unix(1700000000, 0).UTC() })
	if err := w.Emit("run_started", "id1", map[string]any{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "run_started") {
		t.Fatal("missing event")
	}
}

func TestEmitGoldenJSONL(t *testing.T) {
	var b bytes.Buffer
	w := NewWriter(&b)
	w.SetNow(func() time.Time { return time.Unix(1700000000, 0).UTC() })
	_ = w.Emit("run_started", "r1", map[string]any{"cmd": "soak"})
	_ = w.Emit("check_result", "r1", map[string]any{"id": "dns.google.com", "status": "pass"})

	gold := filepath.Join("..", "..", "testdata", "golden", "events.jsonl")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.WriteFile(gold, b.Bytes(), 0o644)
	}
	exp, err := os.ReadFile(gold)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(b.String()) != strings.TrimSpace(string(exp)) {
		t.Fatalf("events jsonl mismatch")
	}
}
