package compare

import (
	"netcheck/internal/model"
	"os"
	"path/filepath"
	"testing"
)

func TestBuild(t *testing.T) {
	before := model.Report{Score: 70, Checks: []model.CheckResult{{ID: "a", Status: model.StatusWarn, DurationMS: 10}}}
	after := model.Report{Score: 90, Checks: []model.CheckResult{{ID: "a", Status: model.StatusPass, DurationMS: 8}}}
	d := Build(before, after)
	if d.AfterScore != 90 || len(d.Items) != 1 {
		t.Fatal("unexpected diff")
	}
}

func TestLoadAndWriteTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.json")
	if err := os.WriteFile(path, []byte(`{"score":10,"checks":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	d := Build(r, r)
	out := filepath.Join(dir, "out.txt")
	if err := WriteTable(out, d); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}
