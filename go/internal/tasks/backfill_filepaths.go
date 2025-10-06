package tasks

import (
	"database/sql"
	"strings"

	"emby-analytics/internal/emby"
	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
)

const legacyFilePathBackfillKey = "legacy_file_path_backfill_done"

// BackfillLegacyFilePaths hydrates missing file_path values for legacy Emby rows so
// the new deduplication logic in stats endpoints reports accurate counts.
func BackfillLegacyFilePaths(db *sql.DB, em *emby.Client, serverID string, serverType media.ServerType) {
	if db == nil || em == nil {
		return
	}
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return
	}

	if done, err := getSettingValue(db, legacyFilePathBackfillKey); err == nil {
		if strings.EqualFold(strings.TrimSpace(done), "true") {
			return
		}
	}

	var missing int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM library_item
		WHERE (file_path IS NULL OR TRIM(file_path) = '')
		  AND (server_id IS NULL OR server_id = '' OR server_id = ?)
	`, serverID).Scan(&missing)
	if err != nil {
		logging.Warn("legacy file_path backfill: failed to count rows", "error", err)
		return
	}
	if missing == 0 {
		_ = setSettingValue(db, legacyFilePathBackfillKey, "true")
		return
	}

	logging.Info("legacy file_path backfill starting", "server_id", serverID, "missing", missing)

	const pageSize = 200
	updated := 0
	page := 0

	for {
		items, err := em.GetItemsChunk(pageSize, page)
		if err != nil {
			logging.Warn("legacy file_path backfill: failed to fetch chunk", "page", page, "error", err)
			break
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			path := strings.TrimSpace(item.FilePath)
			if path == "" {
				continue
			}
			res, err := db.Exec(`
				UPDATE library_item
				SET file_path = ?,
				    server_id = ?,
				    server_type = ?,
				    updated_at = CURRENT_TIMESTAMP
				WHERE id = ?
				  AND (file_path IS NULL OR TRIM(file_path) = '')
			`, path, serverID, string(serverType), item.Id)
			if err != nil {
				logging.Debug("legacy file_path backfill: update failed", "item_id", item.Id, "error", err)
				continue
			}
			if rows, _ := res.RowsAffected(); rows > 0 {
				updated += int(rows)
			}
		}

		if len(items) < pageSize {
			break
		}
		page++
	}

	if updated > 0 {
		logging.Info("legacy file_path backfill completed", "server_id", serverID, "updated", updated)
	}
	_ = setSettingValue(db, legacyFilePathBackfillKey, "true")
}
