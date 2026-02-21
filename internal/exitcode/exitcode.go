package exitcode

import "netcheck/internal/model"

const (
	OK           = 0
	ChecksFailed = 1
	ConfigError  = 2
	RuntimeError = 3
	OutputError  = 4
)

func FromSummary(s model.Summary, strictWarn bool) int {
	if s.Fail > 0 {
		return ChecksFailed
	}
	if strictWarn && s.Warn > 0 {
		return ChecksFailed
	}
	return OK
}
