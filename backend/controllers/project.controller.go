package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ZH1245/go-clockify/backend/models"
	"gorm.io/gorm"
)

// itemHoursSQL turns one task_items row into a number of hours. It is shared by the
// project list and the dashboard so the definition of "hours" lives in exactly one place.
// Running timers count as 0 (end_at and manual_hours are both NULL), matching the UI,
// which adds their live time from start_at. The table must be aliased `ti`.
const itemHoursSQL = `(CASE
	WHEN ti.manual_hours IS NOT NULL THEN ti.manual_hours
	WHEN ti.end_at IS NOT NULL THEN EXTRACT(EPOCH FROM (ti.end_at - ti.start_at)) / 3600.0
	ELSE 0
END)`

// ProjectWithStats is a project plus numbers the UI would otherwise have to compute.
type ProjectWithStats struct {
	models.Project
	TaskCount  int     `json:"task_count"`
	TotalHours float64 `json:"total_hours"`
}

func projectStatsQuery(ctx context.Context, db *gorm.DB) *gorm.DB {
	return db.WithContext(ctx).Raw(`
		SELECT projects.*,
			COUNT(DISTINCT tasks.id) AS task_count,
			COALESCE(SUM(` + itemHoursSQL + `), 0) AS total_hours
		FROM projects
		LEFT JOIN tasks ON tasks.project_id = projects.id
		LEFT JOIN task_items AS ti ON ti.task_id = tasks.id
		GROUP BY projects.id
		ORDER BY projects.created_at DESC`)
}

func GetProjects(ctx context.Context, db *gorm.DB) ([]ProjectWithStats, error) {
	// Start from an empty slice, not a nil one: json.Marshal(nil slice) is "null",
	// which the frontend would have to special-case. [] is friendlier.
	projects := []ProjectWithStats{}
	err := projectStatsQuery(ctx, db).Scan(&projects).Error
	return projects, err
}

func GetProjectByID(ctx context.Context, db *gorm.DB, id uint) (ProjectWithStats, error) {
	var project ProjectWithStats
	err := db.WithContext(ctx).Raw(`
		SELECT projects.*,
			COUNT(DISTINCT tasks.id) AS task_count,
			COALESCE(SUM(`+itemHoursSQL+`), 0) AS total_hours
		FROM projects
		LEFT JOIN tasks ON tasks.project_id = projects.id
		LEFT JOIN task_items AS ti ON ti.task_id = tasks.id
		WHERE projects.id = ?
		GROUP BY projects.id`, id).Scan(&project).Error
	if err != nil {
		return ProjectWithStats{}, err
	}
	// Scan does not report "no rows" as an error, so check the ID ourselves.
	if project.ID == 0 {
		return ProjectWithStats{}, gorm.ErrRecordNotFound
	}
	return project, nil
}

func CreateProject(ctx context.Context, db *gorm.DB, input CreateProjectInput) (models.Project, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return models.Project{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	project := models.Project{Title: title, ClientName: strings.TrimSpace(input.ClientName)}
	if err := db.WithContext(ctx).Create(&project).Error; err != nil {
		return models.Project{}, duplicateProjectTitleError(err)
	}
	return project, nil
}

func UpdateProject(ctx context.Context, db *gorm.DB, id uint, input UpdateProjectInput) (models.Project, error) {
	var project models.Project
	if err := db.WithContext(ctx).First(&project, id).Error; err != nil {
		return models.Project{}, err
	}

	updates := map[string]any{}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return models.Project{}, fmt.Errorf("%w: title cannot be empty", ErrInvalidInput)
		}
		updates["title"] = title
	}
	if input.ClientName != nil {
		updates["client_name"] = strings.TrimSpace(*input.ClientName)
	}
	if len(updates) == 0 {
		return project, nil
	}

	if err := db.WithContext(ctx).Model(&project).Updates(updates).Error; err != nil {
		return models.Project{}, duplicateProjectTitleError(err)
	}
	return project, nil
}

// DeleteProject removes the project but never the tracked time inside it: its tasks
// are kept and simply detached (project_id = NULL). Both steps share one transaction,
// so a failure leaves the project and its tasks exactly as they were.
func DeleteProject(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Task{}).
			Where("project_id = ?", id).
			Updates(map[string]any{"project_id": nil}).Error; err != nil {
			return err
		}

		result := tx.Delete(&models.Project{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func duplicateProjectTitleError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return fmt.Errorf("%w: a project with this title already exists", ErrConflict)
	}
	return err
}
