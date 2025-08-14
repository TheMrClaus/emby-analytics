package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

func Overview(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var usersCount int
		_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&usersCount)

		var itemsCount int
		_ = db.QueryRow(`SELECT COUNT(*) FROM library_item`).Scan(&itemsCount)

		var lifetimeMs int64
		_ = db.QueryRow(`SELECT IFNULL(SUM(total_ms),0) FROM lifetime_watch`).Scan(&lifetimeMs)

		// Convert ms to hours like your Python code
		lifetimeHours := float64(lifetimeMs) / 1000 / 60 / 60

		return c.JSON(fiber.Map{
			"users": usersCount,
			"items": itemsCount,
			"hours": lifetimeHours,
			"ok":    true,
		})
	}
}
