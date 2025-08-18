package admin

import (
	"database/sql"
	"emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
)

// ListUsers shows all users with their IDs
func ListUsers(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get from Emby API
		users, err := em.GetUsers()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Also get from database to see what's synced
		rows, err := db.Query(`SELECT id, name FROM emby_user`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		dbUsers := []fiber.Map{}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				continue
			}
			dbUsers = append(dbUsers, fiber.Map{"id": id, "name": name})
		}

		return c.JSON(fiber.Map{
			"emby_users":     users,
			"database_users": dbUsers,
		})
	}
}
