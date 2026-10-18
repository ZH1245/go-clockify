package models

type Project struct {
	id         int64 "gorm:"
	title      string
	clientName string
}
