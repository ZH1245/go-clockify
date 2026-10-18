package models

import (
	"testing"
	"time"
)

func TestTaskStatusIsValid(t *testing.T) {
	tests := []struct {
		status TaskStatus
		want   bool
	}{
		{TaskStatusTodo, true},
		{TaskStatusInProgress, true},
		{TaskStatusDone, true},
		{"blocked", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.status.IsValid(); got != tt.want {
			t.Errorf("TaskStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestTaskItemTotalHours(t *testing.T) {
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)
	hours := 1.5

	tests := []struct {
		name string
		item TaskItem
		want float64
	}{
		{"manual entry uses manual_hours", TaskItem{StartAt: start, ManualHours: &hours}, 1.5},
		{"timed entry is end minus start", TaskItem{StartAt: start, EndAt: &end}, 1.5},
		{"running entry counts as zero", TaskItem{StartAt: start}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.TotalHours(); got != tt.want {
				t.Errorf("TotalHours() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskTotals(t *testing.T) {
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	hours := 0.5

	task := Task{Items: []TaskItem{
		{StartAt: start, EndAt: &end},         // 2h
		{StartAt: start, ManualHours: &hours}, // 0.5h
		{StartAt: start},                      // running → 0, but flips HasRunningItem
	}}

	if got := task.TotalHours(); got != 2.5 {
		t.Errorf("TotalHours() = %v, want 2.5", got)
	}
	if !task.HasRunningItem() {
		t.Error("HasRunningItem() = false, want true")
	}

	finished := Task{Items: []TaskItem{{StartAt: start, EndAt: &end}}}
	if finished.HasRunningItem() {
		t.Error("HasRunningItem() = true for a finished task, want false")
	}
}
