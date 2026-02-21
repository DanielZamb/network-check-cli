package docs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetTopic(t *testing.T) {
	text, err := Get("run")
	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Fatal("empty man page")
	}
}

func TestUnknownTopic(t *testing.T) {
	_, err := Get("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExport(t *testing.T) {
	d := t.TempDir()
	if err := Export(d); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d, "netcheck-run.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d, "netcheck-run.1")); err != nil {
		t.Fatal(err)
	}
}
