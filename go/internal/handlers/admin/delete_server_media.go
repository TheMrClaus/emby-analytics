package admin

import (
	"database/sql"
	"fmt"
	"strings"

	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
	"emby-analytics/internal/tasks"

	"github.com/gofiber/fiber/v3"
)

const (
	librarySyncSettingPrefix = "library_sync_at_"
	syncInitializedPrefix    = "sync_initialized_"
)

// DeleteServerMedia removes locally stored media metadata for a specific server while keeping watch history intact.
// It clears library_item rows and resets library ingest/sync flags so a fresh sync can repopulate metadata.
func DeleteServerMedia(db *sql.DB, mgr *media.MultiServerManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		serverID := strings.TrimSpace(c.Params("id"))
		if serverID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "server id is required"})
		}

		if mgr != nil {
			if cfgs := mgr.GetServerConfigs(); len(cfgs) > 0 {
				if _, ok := cfgs[serverID]; !ok {
					return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "server not found"})
				}
			}
		}

		tx, err := db.Begin()
		if err != nil {
			logging.Debug("delete media: failed to start transaction", "server_id", serverID, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to start database transaction"})
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		result, err := tx.Exec(`DELETE FROM library_item WHERE server_id = ?`, serverID)
		if err != nil {
			logging.Debug("delete media: failed to clear library items", "server_id", serverID, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete media data"})
		}
		deletedItems, _ := result.RowsAffected()

		clearedSettings := make([]string, 0, 2)
		if _, err := tx.Exec(`DELETE FROM app_settings WHERE key = ?`, librarySyncSettingPrefix+serverID); err != nil {
			logging.Debug("delete media: failed to reset library sync flag", "server_id", serverID, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to reset library sync state"})
		} else {
			clearedSettings = append(clearedSettings, librarySyncSettingPrefix+serverID)
		}

		if _, err := tx.Exec(`DELETE FROM app_settings WHERE key = ?`, syncInitializedPrefix+serverID); err != nil {
			logging.Debug("delete media: failed to reset sync initialized flag", "server_id", serverID, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to reset sync initialization"})
		} else {
			clearedSettings = append(clearedSettings, syncInitializedPrefix+serverID)
		}

		// Clean up unreferenced series entries so they can be rebuilt on the next ingest.
		seriesResult, err := tx.Exec(`DELETE FROM series WHERE id NOT IN (SELECT DISTINCT series_id FROM library_item WHERE series_id IS NOT NULL)`)
		if err != nil {
			logging.Debug("delete media: failed to prune orphan series", "server_id", serverID, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prune orphan series"})
		}
		orphanSeries, _ := seriesResult.RowsAffected()

		if err := tx.Commit(); err != nil {
			logging.Debug("delete media: failed to commit", "server_id", serverID, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to finalize deletion"})
		}
		committed = true
		tasks.ResetServerSyncProgress(serverID)

		logging.Debug("delete media: completed", "server_id", serverID, "deleted_library_items", deletedItems, "cleared_settings", fmt.Sprintf("%v", clearedSettings), "removed_series", orphanSeries)

		return c.JSON(fiber.Map{
			"success":               true,
			"server_id":             serverID,
			"deleted_library_items": deletedItems,
			"removed_orphan_series": orphanSeries,
			"cleared_settings":      clearedSettings,
		})
	}
}
