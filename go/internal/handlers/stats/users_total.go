package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

func UsersTotal(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		total := 0
		_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&total)
		return c.JSON(fiber.Map{"total_users": total})
	}
}
