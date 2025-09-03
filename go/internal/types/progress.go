package types

import (
	"database/sql"
	"emby-analytics/internal/emby"
)

// Progress represents the status of a refresh operation
type Progress struct {
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
	Done      bool   `json:"done"`
	Running   bool   `json:"running"`
	Page      int    `json:"page"`
}

// RefreshManager interface defines the methods needed by the scheduler
type RefreshManager interface {
	Get() Progress
	Start(db *sql.DB, em *emby.Client, chunkSize int)
	StartIncremental(db *sql.DB, em *emby.Client)
}