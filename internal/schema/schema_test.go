package schema

import (
	"encoding/json"
	"netcheck/internal/model"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("schema version must not be empty")
	}
}

func TestReportSchemaRequiredFields(t *testing.T) {
	r := model.Report{
		SchemaVersion: Version,
		Timestamp:     time.Unix(0, 0).UTC(),
		Host:          "h",
		OS:            "darwin",
		Version:       "dev",
		Config:        map[string]any{},
		Checks:        []model.CheckResult{},
		Summary:       model.Summary{},
		Score:         0,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "timestamp", "host", "os", "version", "config", "checks", "summary", "score"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing required key %q", key)
		}
	}
}
