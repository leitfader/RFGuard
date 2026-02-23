package ingest

import (
	"context"
	"log/slog"
	"time"

	"rfguard/internal/model"
)

func SendNonBlocking(ctx context.Context, out chan<- model.NormalizedEvent, ev model.NormalizedEvent, logger *slog.Logger) bool {
	select {
	case out <- ev:
		return true
	case <-ctx.Done():
		return false
	default:
		if logger != nil {
			logger.Warn("event channel full, dropping event", "reader_id", ev.ReaderID, "timestamp", ev.Timestamp)
		}
		return false
	}
}

func BackoffSleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		d = 200 * time.Millisecond
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
