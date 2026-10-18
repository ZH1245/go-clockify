package controllers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ZH1245/go-clockify/backend/models"
	"gorm.io/gorm"
)

func GetTasks(db *gorm.DB) ([]models.Task, error) {
	var tasks []models.Task
	err := db.Preload("Items", newestItemsFirst).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func GetTaskByID(db *gorm.DB, id uint) (models.Task, error) {
	var task models.Task
	err := db.Preload("Items", newestItemsFirst).First(&task, id).Error
	return task, err
}

func CreateTask(db *gorm.DB, input CreateTaskInput) (models.Task, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return models.Task{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	task := models.Task{Title: title, Status: models.TaskStatusTodo}
	if err := db.Create(&task).Error; err != nil {
		return models.Task{}, duplicateTitleError(err)
	}
	return task, nil
}

func UpdateTask(db *gorm.DB, id uint, input UpdateTaskInput) (models.Task, error) {
	task, err := GetTaskByID(db, id)
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

	if err := db.Model(&task).Updates(updates).Error; err != nil {
		return models.Task{}, duplicateTitleError(err)
	}
	return GetTaskByID(db, id)
}

// DeleteTask also deletes the task's items (ON DELETE CASCADE in the database).
func DeleteTask(db *gorm.DB, id uint) error {
	result := db.Delete(&models.Task{}, id)
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
