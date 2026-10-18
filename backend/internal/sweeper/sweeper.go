package sweeper

import (
	"context"
	"log"
	"time"

	"github.com/ZH1245/go-clockify/backend/controllers"
	"gorm.io/gorm"
)

// Start runs the stale-timer sweep in a background goroutine and returns immediately.
//
// This is the third shape of concurrency in this project, after context (one per request)
// and WaitGroup + channel (one per dashboard call): a long-lived goroutine that owns its
// own lifetime. There is no channel and no WaitGroup because nobody is waiting for a
// result - no request is blocked on this. The goroutine's only coordination is ctx.Done(),
// which main cancels on shutdown.
//
// interval <= 0 disables the sweeper, which is handy when you want to test abandoned
// timers by hand.
func Start(ctx context.Context, db *gorm.DB, interval, timeout time.Duration) {
	if interval <= 0 || timeout <= 0 {
		log.Printf("timer sweeper: disabled (interval=%s timeout=%s)", interval, timeout)
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Sweep once at startup rather than waiting a full interval: timers that were left
		// running while the server was down get closed now.
		sweep(ctx, db, timeout)

		for {
			select {
			case <-ctx.Done():
				return // shutdown: the deferred ticker.Stop() releases the ticker
			case <-ticker.C:
				sweep(ctx, db, timeout)
			}
		}
	}()
}

// sweep gives each run its own budget, so one slow query cannot stall the next tick.
func sweep(ctx context.Context, db *gorm.DB, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stopped, err := controllers.SweepStaleTimers(ctx, db, timeout)
	if err != nil {
		log.Printf("timer sweeper: %v", err)
		return
	}
	if stopped > 0 {
		log.Printf("timer sweeper: stopped %d timer(s) with no recent ping", stopped)
	}
}
