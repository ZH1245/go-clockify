package controllers

import (
	"errors"
	"time"

	"github.com/ZH1245/go-clockify/backend/models"
)

// Routes turn these into HTTP status codes: ErrInvalidInput → 400, ErrConflict → 409.
// A missing row comes back as gorm.ErrRecordNotFound → 404.
var (
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("conflict")
)

type CreateTaskInput struct {
	Title string `json:"title"`
}

// Nil fields are left unchanged.
type UpdateTaskInput struct {
	Title  *string            `json:"title"`
	Status *models.TaskStatus `json:"status"`
}

// ManualEntryInput is a time entry typed in by hand: when the work happened, and how long.
type ManualEntryInput struct {
	StartAt     time.Time `json:"start_at"`
	ManualHours float64   `json:"manual_hours"`
}

// UpdateTaskItemInput replaces an entry's times. Send EndAt for a timed entry, or ManualHours for a manual one.
type UpdateTaskItemInput struct {
	StartAt     time.Time  `json:"start_at"`
	EndAt       *time.Time `json:"end_at"`
	ManualHours *float64   `json:"manual_hours"`
}
