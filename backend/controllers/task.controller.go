package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ZH1245/go-clockify/backend/models"
	"gorm.io/gorm"
)

// Every controller takes a context.Context first and hands it to db.WithContext(ctx).
// The HTTP layer passes r.Context(), which is cancelled as soon as the client goes away
// (or when the server shuts down), so a slow query stops instead of running to the end.
// db.WithContext also carries the request's deadline down to the database driver.

func GetTasks(ctx context.Context, db *gorm.DB) ([]models.Task, error) {
	var tasks []models.Task
	err := db.WithContext(ctx).
		Preload("Project").
		Preload("Items", newestItemsFirst).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

func GetTaskByID(ctx context.Context, db *gorm.DB, id uint) (models.Task, error) {
	var task models.Task
	err := db.WithContext(ctx).
		Preload("Project").
		Preload("Items", newestItemsFirst).
		First(&task, id).Error
	return task, err
}

func CreateTask(ctx context.Context, db *gorm.DB, input CreateTaskInput) (models.Task, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return models.Task{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	task := models.Task{Title: title, Status: models.TaskStatusTodo, ProjectID: input.ProjectID}
	if input.ProjectID != nil {
		if err := db.WithContext(ctx).First(&models.Project{}, *input.ProjectID).Error; err != nil {
			return models.Task{}, err
		}
	}
	if err := db.WithContext(ctx).Create(&task).Error; err != nil {
		return models.Task{}, duplicateTitleError(err)
	}
	return GetTaskByID(ctx, db, task.ID)
}

func UpdateTask(ctx context.Context, db *gorm.DB, id uint, input UpdateTaskInput) (models.Task, error) {
	task, err := GetTaskByID(ctx, db, id)
	if err != nil {
		return models.Task{}, err
	}

	updates := map[string]any{}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return models.Task{}, fmt.Errorf("%w: title cannot be empty", ErrInvalidInput)
		}
		updates["title"] = title
	}
	if input.Status != nil {
		if !input.Status.IsValid() {
			return models.Task{}, fmt.Errorf("%w: status must be todo, in_progress or done", ErrInvalidInput)
		}
		updates["status"] = *input.Status
	}
	if len(updates) == 0 {
		return task, nil
	}

	if err := db.WithContext(ctx).Model(&task).Updates(updates).Error; err != nil {
		return models.Task{}, duplicateTitleError(err)
	}
	return GetTaskByID(ctx, db, id)
}

// SetTaskProject points a task at a project, or clears it when projectID is nil.
// This is a separate call from UpdateTask because "no project_id field in the JSON"
// (leave unchanged) and "project_id: null" (clear) look the same on a *uint.
func SetTaskProject(ctx context.Context, db *gorm.DB, taskID uint, projectID *uint) (models.Task, error) {
	if _, err := GetTaskByID(ctx, db, taskID); err != nil {
		return models.Task{}, err
	}

	if projectID != nil {
		var project models.Project
		if err := db.WithContext(ctx).First(&project, *projectID).Error; err != nil {
			return models.Task{}, err
		}
	}

	// Updates with a map writes every key, including a nil value, so this stores NULL
	// when no project is given (an empty struct would skip the zero value instead).
	updates := map[string]any{"project_id": nil}
	if projectID != nil {
		updates["project_id"] = *projectID
	}
	if err := db.WithContext(ctx).Model(&models.Task{ID: taskID}).Updates(updates).Error; err != nil {
		return models.Task{}, err
	}
	return GetTaskByID(ctx, db, taskID)
}

// DeleteTask also deletes the task's items (ON DELETE CASCADE in the database).
func DeleteTask(ctx context.Context, db *gorm.DB, id uint) error {
	result := db.WithContext(ctx).Delete(&models.Task{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func newestItemsFirst(db *gorm.DB) *gorm.DB {
	return db.Order("start_at DESC")
}

func duplicateTitleError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return fmt.Errorf("%w: a task with this title already exists", ErrConflict)
	}
	return err
}
