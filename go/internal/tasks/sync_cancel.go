package tasks

import (
	"database/sql"
	"errors"

	"emby-analytics/internal/handlers/settings"
)

// ErrSyncCancelled indicates a sync was cancelled by disabling sync for the server.
var ErrSyncCancelled = errors.New("sync cancelled by user")

const cancelCheckInterval = 50

func isSyncDisabled(db *sql.DB, serverID string, defaultEnabled bool) bool {
	return !settings.GetSyncEnabled(db, serverID, defaultEnabled)
}
