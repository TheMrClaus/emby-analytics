package admin

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/tasks"
)

// POST /admin/users/sync -> { started: true }
func UsersSyncHandler(db *sql.DB, em *emby.Client, cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		// fire-and-forget single cycle
		go tasks.RunOnce(db, em, cfg)
		return c.JSON(fiber.Map{"started": true})
	}
}
