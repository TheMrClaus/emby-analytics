package tasks

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
	"fmt"
	"strings"
)

// CleanupOrphanedServerItems removes library items that belong to servers not currently configured.
// This repairs database state if a server was removed or if garbage data corrupted server_id columns.
func CleanupOrphanedServerItems(db *sql.DB, mgr *media.MultiServerManager) {
	configs := mgr.GetServerConfigs()
	if len(configs) == 0 {
		return
	}

	validIDs := make([]string, 0, len(configs))
	for id := range configs {
		validIDs = append(validIDs, id)
	}

	// Double check: assume "default-emby" is always valid if we used it historically?
	// But GetServerConfigs should return all valid ones.

	// Construct placeholders
	placeholders := strings.Repeat("?,", len(validIDs))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf("DELETE FROM library_item WHERE server_id NOT IN (%s)", placeholders)

	args := make([]interface{}, len(validIDs))
	for i, id := range validIDs {
		args[i] = id
	}

	result, err := db.Exec(query, args...)
	if err != nil {
		logging.Warn("Failed to cleanup orphaned server items", "error", err)
		return
	}

	// ... existing logic ...
	rows, _ := result.RowsAffected()
	if rows > 0 {
		logging.Info("Cleaned up orphaned library items", "count", rows, "valid_servers", validIDs)
	}

	CleanupOrphanedSeries(db)
}

// CleanupOrphanedSeries removes series that have no remaining library items associated with them.
func CleanupOrphanedSeries(db *sql.DB) {
	// Delete series that are not referenced by any library_item (as series_id) 
	// AND not referenced by any library_item (as the item itself, though typically series table IS the metadata source)
	// Generally, if no library_item has series_id = X, then Series X is empty/gone.
	query := `
		DELETE FROM series 
		WHERE id NOT IN (
			SELECT DISTINCT series_id 
			FROM library_item 
			WHERE series_id IS NOT NULL AND series_id != ''
		)
	`
	
	result, err := db.Exec(query)
	if err != nil {
		logging.Warn("Failed to cleanup orphaned series", "error", err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		logging.Info("Cleaned up orphaned series records", "count", rows)
	}
}
