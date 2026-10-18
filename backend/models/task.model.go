package models

import "time"

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

func (status TaskStatus) IsValid() bool {
	switch status {
	case TaskStatusTodo, TaskStatusInProgress, TaskStatusDone:
		return true
	}
	return false
}

type Task struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Title     string     `gorm:"not null;uniqueIndex" json:"title"`
	Status    TaskStatus `gorm:"type:varchar(20);not null;default:todo" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	// ProjectID is a pointer so a task can have no project (NULL), and so "leave it
	// alone" (nil in an update) is different from "set it to empty".
	// The foreign key is left as NO ACTION on purpose: Postgres refuses to delete a
	// project that still has tasks, so deletes must go through DeleteProject, which
	// detaches the tasks first. See controllers/project.controller.go.
	ProjectID *uint      `gorm:"index" json:"project_id"`
	Project   *Project   `json:"project,omitempty"`
	Items     []TaskItem `gorm:"constraint:OnDelete:CASCADE" json:"items"`
}

// TotalHours sums the finished items; Items must be loaded first (db.Preload("Items")).
// A running item counts as 0, so the UI shows its live time from StartAt.
func (task Task) TotalHours() float64 {
	var total float64
	for _, item := range task.Items {
		total += item.TotalHours()
	}
	return total
}

func (task Task) HasRunningItem() bool {
	for _, item := range task.Items {
		if item.IsRunning() {
			return true
		}
	}
	return false
}

// TaskItem is one time entry for a Task, either timed (StartAt → EndAt, set by start/stop)
// or manual (StartAt = when the work happened, ManualHours = how long). A timed entry with
// no EndAt yet is a running timer.
type TaskItem struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TaskID      uint       `gorm:"not null;index" json:"task_id"`
	StartAt     time.Time  `gorm:"not null" json:"start_at"`
	EndAt       *time.Time `json:"end_at"`
	ManualHours *float64   `gorm:"type:numeric(6,2)" json:"manual_hours"`
	// LastPingAt is refreshed by POST /timers/ping while a browser tab is open. A timer
	// whose last ping is older than the timeout is stopped by the sweeper, so closing the
	// tab (or losing the browser) does not leave a timer running forever.
	LastPingAt *time.Time `json:"last_ping_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (item TaskItem) IsRunning() bool {
	return item.EndAt == nil && item.ManualHours == nil
}

func (item TaskItem) TotalHours() float64 {
	if item.ManualHours != nil {
		return *item.ManualHours
	}
	if item.EndAt == nil {
		return 0
	}
	return item.EndAt.Sub(item.StartAt).Hours()
}
