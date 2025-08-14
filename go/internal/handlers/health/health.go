package health

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

func Health(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	}
}
