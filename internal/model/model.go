package model

import "time"

type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

const SchemaVersion = "v1"

type Report struct {
	SchemaVersion string                 `json:"schema_version"`
	Timestamp     time.Time              `json:"timestamp"`
	Host          string                 `json:"host"`
	OS            string                 `json:"os"`
	Version       string                 `json:"version"`
	GitCommit     string                 `json:"git_commit,omitempty"`
	RunID         string                 `json:"run_id,omitempty"`
	Labels        map[string]string      `json:"labels,omitempty"`
	Config        map[string]any         `json:"config"`
	Checks        []CheckResult          `json:"checks"`
	Summary       Summary                `json:"summary"`
	Score         int                    `json:"score"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type CheckResult struct {
	ID         string         `json:"id"`
	Group      string         `json:"group"`
	Target     string         `json:"target,omitempty"`
	Status     Status         `json:"status"`
	Metrics    map[string]any `json:"metrics,omitempty"`
	Raw        string         `json:"raw,omitempty"`
	DurationMS int64          `json:"duration_ms"`
	Error      string         `json:"error,omitempty"`
}

type Summary struct {
	Pass  int `json:"pass"`
	Warn  int `json:"warn"`
	Fail  int `json:"fail"`
	Skip  int `json:"skip"`
	Total int `json:"total"`
}

type RunOptions struct {
	Format      string
	OutPath     string
	Verbose     bool
	Quiet       bool
	NoColor     bool
	FailFast    bool
	TimeoutSec  int
	Select      []string
	Skip        []string
	RunID       string
	Labels      map[string]string
	StrictWarn  bool
	ConfigPath  string
	CommandName string
}

func (s *Summary) Add(status Status) {
	s.Total++
	switch status {
	case StatusPass:
		s.Pass++
	case StatusWarn:
		s.Warn++
	case StatusFail:
		s.Fail++
	case StatusSkip:
		s.Skip++
	}
}
