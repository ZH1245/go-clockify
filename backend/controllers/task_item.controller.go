package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/ZH1245/go-clockify/backend/models"
	"gorm.io/gorm"
)

const (
	runningItems   = "end_at IS NULL AND manual_hours IS NULL"
	maxManualHours = 24.0
)

// StartTimer creates a running entry for the task. Several tasks may run at once, but each task has
// at most one running timer, so its time is never counted twice. The transaction makes the entry and
// the status change succeed or fail together.
func StartTimer(ctx context.Context, db *gorm.DB, taskID uint) (models.TaskItem, error) {
	var item models.TaskItem
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task models.Task
		if err := tx.First(&task, taskID).Error; err != nil {
			return err
		}

		var running int64
		if err := tx.Model(&models.TaskItem{}).Where("task_id = ? AND "+runningItems, taskID).Count(&running).Error; err != nil {
			return err
		}
		if running > 0 {
			return fmt.Errorf("%w: this task already has a running timer", ErrConflict)
		}

		// Seeded with the first ping: a timer with no ping at all would never be swept.
		now := time.Now().UTC()
		item = models.TaskItem{TaskID: taskID, StartAt: now, LastPingAt: &now}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return tx.Model(&task).Update("status", models.TaskStatusInProgress).Error
	})
	return item, err
}

func StopTimer(ctx context.Context, db *gorm.DB, itemID uint) (models.TaskItem, error) {
	var item models.TaskItem
	if err := db.WithContext(ctx).First(&item, itemID).Error; err != nil {
		return models.TaskItem{}, err
	}
	if !item.IsRunning() {
		return models.TaskItem{}, fmt.Errorf("%w: this timer is not running", ErrConflict)
	}

	now := time.Now().UTC()
	if err := db.WithContext(ctx).Model(&item).Update("end_at", now).Error; err != nil {
		return models.TaskItem{}, err
	}
	item.EndAt = &now
	return item, nil
}

func AddManualEntry(ctx context.Context, db *gorm.DB, taskID uint, input ManualEntryInput) (models.TaskItem, error) {
	if input.StartAt.IsZero() {
		return models.TaskItem{}, fmt.Errorf("%w: start_at is required", ErrInvalidInput)
	}
	if err := validateManualHours(input.ManualHours); err != nil {
		return models.TaskItem{}, err
	}
	if err := db.WithContext(ctx).First(&models.Task{}, taskID).Error; err != nil {
		return models.TaskItem{}, err
	}

	hours := input.ManualHours
	item := models.TaskItem{TaskID: taskID, StartAt: input.StartAt.UTC(), ManualHours: &hours}
	if err := db.WithContext(ctx).Create(&item).Error; err != nil {
		return models.TaskItem{}, err
	}
	return item, nil
}

func UpdateTaskItem(ctx context.Context, db *gorm.DB, itemID uint, input UpdateTaskItemInput) (models.TaskItem, error) {
	if err := validateEntryUpdate(input); err != nil {
		return models.TaskItem{}, err
	}

	var item models.TaskItem
	if err := db.WithContext(ctx).First(&item, itemID).Error; err != nil {
		return models.TaskItem{}, err
	}

	changes := models.TaskItem{StartAt: input.StartAt.UTC(), EndAt: input.EndAt, ManualHours: input.ManualHours}
	// Select lists the columns to write, so a nil EndAt or ManualHours is saved as NULL instead of skipped.
	if err := db.WithContext(ctx).Model(&item).Select("start_at", "end_at", "manual_hours").Updates(changes).Error; err != nil {
		return models.TaskItem{}, err
	}

	err := db.WithContext(ctx).First(&item, itemID).Error
	return item, err
}

func DeleteTaskItem(ctx context.Context, db *gorm.DB, itemID uint) error {
	result := db.WithContext(ctx).Delete(&models.TaskItem{}, itemID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func validateManualHours(hours float64) error {
	if hours <= 0 || hours > maxManualHours {
		return fmt.Errorf("%w: manual_hours must be more than 0 and at most %.0f", ErrInvalidInput, maxManualHours)
	}
	return nil
}

// An entry is timed (EndAt), manual (ManualHours) or still running (neither), never both.
func validateEntryUpdate(input UpdateTaskItemInput) error {
	if input.StartAt.IsZero() {
		return fmt.Errorf("%w: start_at is required", ErrInvalidInput)
	}
	if input.EndAt != nil && input.ManualHours != nil {
		return fmt.Errorf("%w: send either end_at or manual_hours, not both", ErrInvalidInput)
	}
	if input.EndAt != nil && !input.EndAt.After(input.StartAt) {
		return fmt.Errorf("%w: end_at must be after start_at", ErrInvalidInput)
	}
	if input.ManualHours != nil {
		return validateManualHours(*input.ManualHours)
	}
	return nil
}
