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
	Title     string `json:"title"`
	ProjectID *uint  `json:"project_id"`
}

// Nil fields are left unchanged.
type UpdateTaskInput struct {
	Title  *string            `json:"title"`
	Status *models.TaskStatus `json:"status"`
}

// SetTaskProjectInput is the body of PUT /tasks/{id}/project. Send a number to move the
// task, or JSON null to take it out of its project.
type SetTaskProjectInput struct {
	ProjectID *uint `json:"project_id"`
}

type CreateProjectInput struct {
	Title      string `json:"title"`
	ClientName string `json:"client_name"`
}

// Nil fields are left unchanged.
type UpdateProjectInput struct {
	Title      *string `json:"title"`
	ClientName *string `json:"client_name"`
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
