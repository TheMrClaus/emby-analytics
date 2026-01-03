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

	rows, _ := result.RowsAffected()
	if rows > 0 {
		logging.Info("Cleaned up orphaned library items", "count", rows, "valid_servers", validIDs)
	}
}
