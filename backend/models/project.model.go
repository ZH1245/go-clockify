package models

import "time"

// Project groups tasks for a client. Deleting a project keeps its tasks and their
// tracked time; the tasks are only detached (project_id = NULL) by DeleteProject.
type Project struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Title is unique. ClientName is only indexed, not unique: many projects can have
	// no client, and a unique index would reject the second empty string.
	Title      string    `gorm:"not null;uniqueIndex" json:"title"`
	ClientName string    `gorm:"index" json:"client_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Tasks      []Task    `json:"tasks,omitempty"`
}
