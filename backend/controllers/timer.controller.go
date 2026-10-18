package controllers

import (
	"context"
	"time"

	"github.com/ZH1245/go-clockify/backend/models"
	"gorm.io/gorm"
)

// PingTimers records "the browser is still here" for every running timer.
//
// The frontend POSTs /timers/ping on a short interval while a tab is open. The sweeper
// (see SweepStaleTimers) treats a timer that stops being pinged as abandoned and stops it.
func PingTimers(ctx context.Context, db *gorm.DB) (int64, error) {
	result := db.WithContext(ctx).
		Model(&models.TaskItem{}).
		Where(runningItems).
		Update("last_ping_at", time.Now().UTC())
	return result.RowsAffected, result.Error
}

// SweepStaleTimers stops timers that have not been pinged for longer than timeout.
//
// The staleness check and the write are one SQL statement, so there is no gap between
// "read the row" and "update the row" that a ping could slip into. Either the ping lands
// first and the row is skipped, or it lands second and finds end_at already set (so its
// own WHERE clause matches nothing). Both orderings are correct, with no lock needed.
//
// end_at is set to last_ping_at, not to now(). The user stopped working when the browser
// went away, so the time between the last ping and this sweep must not be counted. A row
// with last_ping_at IS NULL is never swept: "never pinged" is not "pinged too long ago",
// and old rows from before this feature existed keep running rather than silently closing.
func SweepStaleTimers(ctx context.Context, db *gorm.DB, timeout time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-timeout)
	result := db.WithContext(ctx).
		Model(&models.TaskItem{}).
		Where(runningItems+" AND last_ping_at IS NOT NULL AND last_ping_at < ?", cutoff).
		Update("end_at", gorm.Expr("last_ping_at"))
	return result.RowsAffected, result.Error
}
