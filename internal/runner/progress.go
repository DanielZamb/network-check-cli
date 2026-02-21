package runner

import (
	"context"
	"netcheck/internal/model"
)

type ProgressEvent struct {
	Phase  string
	Check  string
	Group  string
	Index  int
	Total  int
	Status model.Status
}

type ProgressReporter func(ProgressEvent)

type progressKey struct{}

func WithProgressReporter(ctx context.Context, reporter ProgressReporter) context.Context {
	if reporter == nil {
		return ctx
	}
	return context.WithValue(ctx, progressKey{}, reporter)
}

func reportProgress(ctx context.Context, ev ProgressEvent) {
	if reporter, ok := ctx.Value(progressKey{}).(ProgressReporter); ok && reporter != nil {
		reporter(ev)
	}
}
