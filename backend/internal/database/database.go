package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(databaseURL string) (*gorm.DB, error) {
	// TranslateError turns Postgres error codes into GORM errors, e.g. a unique-index violation
	// into gorm.ErrDuplicatedKey, so controllers can check for it with errors.Is.
	return gorm.Open(postgres.Open(databaseURL), &gorm.Config{TranslateError: true})
}

// Migrate creates or updates tables for the given models, e.g. Migrate(db, &models.Task{}).
func Migrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}
