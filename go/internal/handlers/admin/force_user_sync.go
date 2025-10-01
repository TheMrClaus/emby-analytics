package admin

import (
	"database/sql"

	"emby-analytics/internal/media"
	"emby-analytics/internal/tasks"

	"github.com/gofiber/fiber/v3"
)

// ForceUserSync forces an immediate user sync and returns results
func ForceUserSync(db *sql.DB, mgr *media.MultiServerManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Run sync immediately (blocking)
		tasks.RunUserSyncOnce(db, mgr)

		// Get current count
		var totalUsers int
		db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&totalUsers)

		return c.JSON(fiber.Map{
			"success":     true,
			"message":     "User sync completed",
			"total_users": totalUsers,
		})
	}
}
