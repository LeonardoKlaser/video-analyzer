package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/leoklaser/video-analyzer/api/internal/db"
)

// StartWatchdog runs every minute marking analyses stuck in 'processing'
// for more than 8 minutes as 'error'. Returns when ctx is canceled.
func StartWatchdog(ctx context.Context, d *db.DB) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			ids, err := db.StaleProcessing(tickCtx, d, 8)
			if err != nil {
				slog.Error("watchdog: query stale", "err", err)
				cancel()
				continue
			}
			for _, id := range ids {
				if err := db.SetError(tickCtx, d, id, "Análise interrompida (timeout)"); err != nil {
					slog.Error("watchdog: mark error", "id", id, "err", err)
				} else {
					slog.Warn("watchdog: marked stale job as error", "id", id)
				}
			}
			cancel()
		}
	}
}
