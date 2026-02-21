package checks

import (
	"context"
	"netcheck/internal/config"
	"netcheck/internal/execx"
	"netcheck/internal/model"
)

type Check interface {
	ID() string
	Group() string
	Run(ctx context.Context, exec execx.Executor, cfg config.Config, timeoutSec int) model.CheckResult
}
