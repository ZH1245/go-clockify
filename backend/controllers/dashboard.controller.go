package controllers

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

// RunningTimer is one active timer, joined with the task and project it belongs to so
// the UI can render a row without extra requests.
type RunningTimer struct {
	ItemID       uint      `json:"item_id"`
	TaskID       uint      `json:"task_id"`
	TaskTitle    string    `json:"task_title"`
	ProjectID    *uint     `json:"project_id"`
	ProjectTitle *string   `json:"project_title"`
	StartAt      time.Time `json:"start_at"`
}

type DashboardTotals struct {
	TodayHours   float64 `json:"today_hours"`
	WeekHours    float64 `json:"week_hours"`
	AllTimeHours float64 `json:"all_time_hours"`
}

type Dashboard struct {
	RunningTimers []RunningTimer  `json:"running_timers"`
	Totals        DashboardTotals `json:"totals"`
}

const runningTimersSQL = `
	SELECT ti.id AS item_id,
		ti.task_id,
		tasks.title AS task_title,
		tasks.project_id,
		projects.title AS project_title,
		ti.start_at
	FROM task_items AS ti
	JOIN tasks ON tasks.id = ti.task_id
	LEFT JOIN projects ON projects.id = tasks.project_id
	WHERE ti.end_at IS NULL AND ti.manual_hours IS NULL
	ORDER BY ti.start_at ASC`

const dashboardTotalsSQL = `
	SELECT
		COALESCE(SUM(` + itemHoursSQL + `) FILTER (WHERE ti.start_at >= ?), 0) AS today_hours,
		COALESCE(SUM(` + itemHoursSQL + `) FILTER (WHERE ti.start_at >= ?), 0) AS week_hours,
		COALESCE(SUM(` + itemHoursSQL + `), 0) AS all_time_hours
	FROM task_items AS ti
	WHERE ti.end_at IS NOT NULL OR ti.manual_hours IS NOT NULL`

// GetDashboard builds the whole dashboard. The two queries do not depend on each other,
// so they run at the same time instead of one after the other.
//
// This is the standard-library way to fan out to a few goroutines and wait for them:
//
//   - sync.WaitGroup counts the goroutines. Add(n) BEFORE starting them, and every
//     goroutine does defer wg.Done() so the counter drops even on an early return.
//     wg.Wait() blocks until the counter is back to zero.
//   - a channel collects the results. It is buffered with exactly one slot per
//     goroutine, so a goroutine can always send without waiting for a receiver;
//     we drain it only after wg.Wait() has confirmed every send has happened.
//
// The two goroutines write to two different variables (running, totals), which is safe.
// Two goroutines writing the same variable would be a data race.
func GetDashboard(ctx context.Context, db *gorm.DB) (Dashboard, error) {
	var (
		running []RunningTimer
		totals  DashboardTotals
	)

	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekdayOffset := (int(startOfDay.Weekday()) + 6) % 7 // Monday is 0
	startOfWeek := startOfDay.AddDate(0, 0, -weekdayOffset)

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- db.WithContext(ctx).Raw(runningTimersSQL).Scan(&running).Error
	}()
	go func() {
		defer wg.Done()
		errs <- db.WithContext(ctx).Raw(dashboardTotalsSQL, startOfDay, startOfWeek).Scan(&totals).Error
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return Dashboard{}, err
		}
	}

	// JSON should be [] rather than null when nothing is running.
	if running == nil {
		running = []RunningTimer{}
	}
	return Dashboard{RunningTimers: running, Totals: totals}, nil
}
