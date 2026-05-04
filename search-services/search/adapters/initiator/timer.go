package initiator

import (
	"context"
	"log/slog"
	"time"

	"yadro.com/course/search/core"
)

type Timer struct {
	svc    core.Searcher
	ttl    time.Duration
	logger *slog.Logger
}

func NewTimer(svc core.Searcher, ttl time.Duration, logger *slog.Logger) *Timer {
	return &Timer{svc: svc, ttl: ttl, logger: logger}
}

func (t *Timer) Start(ctx context.Context) {
	if err := t.svc.BuildIndex(ctx); err != nil {
		t.logger.Error("initial index build failed", "error", err)
	}

	ticker := time.NewTicker(t.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := t.svc.BuildIndex(ctx); err != nil {
				t.logger.Error("periodic index update failed", "error", err)
			}
		}
	}
}
