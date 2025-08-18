package admin

import (
	"database/sql"
	"github.com/gofiber/fiber/v3"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/tasks"
)

// ResetAllData clears all data and re-syncs from scratch
func ResetAllData(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Clear all tables
		tables := []string{"play_event", "lifetime_watch", "emby_user", "library_item"}
		deleted := make(map[string]int64)
		
		for _, table := range tables {
			result, err := db.Exec(`DELETE FROM ` + table)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to clear " + table + ": " + err.Error()})
			}
			rows, _ := result.RowsAffected()
			deleted[table] = rows
		}

		// Re-sync users immediately
		tasks.RunUserSyncOnce(db, em)
		
		// Get final counts
		var finalUsers int
		db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&finalUsers)

		return c.JSON(fiber.Map{
			"success": true,
			"message": "All data cleared and re-synced",
			"deleted_records": deleted,
			"final_user_count": finalUsers,
		})
	}
}
