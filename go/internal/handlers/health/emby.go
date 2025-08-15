package health

import (
	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

// GET /health/emby
func Emby(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// lightweight: try a tiny Items call (limit 1) using the existing client
		_, err := em.TotalItems()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"ok": false, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}
