package sync

import (
	"database/sql"
	"time"
)

// SyncType constants for different sync operations
const (
	SyncTypeLibraryIncremental = "library_incremental"
	SyncTypeLibraryFull        = "library_full"
)

// GetLastSyncTime retrieves the last sync timestamp for a given sync type
func GetLastSyncTime(db *sql.DB, syncType string) (*time.Time, error) {
	var lastSync time.Time
	err := db.QueryRow(`
		SELECT last_sync_at FROM sync_tracking 
		WHERE sync_type = ?
	`, syncType).Scan(&lastSync)

	if err != nil {
		if err == sql.ErrNoRows {
			// No previous sync, return epoch time
			epoch := time.Unix(0, 0)
			return &epoch, nil
		}
		return nil, err
	}

	return &lastSync, nil
}

// UpdateSyncTime updates the last sync timestamp and item count for a sync type
func UpdateSyncTime(db *sql.DB, syncType string, itemsProcessed int) error {
	now := time.Now().UTC()

	_, err := db.Exec(`
		INSERT INTO sync_tracking (sync_type, last_sync_at, items_processed, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(sync_type) DO UPDATE SET
			last_sync_at = excluded.last_sync_at,
			items_processed = excluded.items_processed,
			updated_at = excluded.updated_at
	`, syncType, now, itemsProcessed, now)

	return err
}

// GetSyncStats retrieves sync statistics for a given sync type
func GetSyncStats(db *sql.DB, syncType string) (lastSync time.Time, itemsProcessed int, err error) {
	err = db.QueryRow(`
		SELECT last_sync_at, items_processed 
		FROM sync_tracking 
		WHERE sync_type = ?
	`, syncType).Scan(&lastSync, &itemsProcessed)

	return lastSync, itemsProcessed, err
}
