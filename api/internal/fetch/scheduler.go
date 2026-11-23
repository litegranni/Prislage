package fetch

import (
	"context"
	"log/slog"
	"time"
)

// Runner is implemented by service.Service.FetchAllAuto via adapter.
type AutoFetcher interface {
	FetchAllAuto(ctx context.Context, ratePerMinute int) (ok, fail int)
}

// Scheduler runs AutoFetcher once per interval inside the API process.
type Scheduler struct {
	Fetcher       AutoFetcher
	Interval      time.Duration
	RatePerMinute int
	Log           *slog.Logger
}

func (s *Scheduler) Start(ctx context.Context) {
	if s.Interval <= 0 {
		s.Interval = 24 * time.Hour
	}
	if s.Log == nil {
		s.Log = slog.Default()
	}
	go func() {
		// Small delay so migrations/boot settle.
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.runOnce(ctx)
				timer.Reset(s.Interval)
			}
		}
	}()
}

func (s *Scheduler) runOnce(ctx context.Context) {
	s.Log.Info("scheduled fetch starting")
	ok, fail := s.Fetcher.FetchAllAuto(ctx, s.RatePerMinute)
	s.Log.Info("scheduled fetch done", "ok", ok, "fail", fail)
}
