package admin

import (
	"database/sql"
	"github.com/gofiber/fiber/v3"
)

// CleanupUsers removes users with empty/invalid IDs
func CleanupUsers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Find invalid users
		rows, err := db.Query(`SELECT id, name FROM emby_user WHERE id = '' OR id IS NULL`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		invalidUsers := []fiber.Map{}
		for rows.Next() {
			var id, name sql.NullString
			if err := rows.Scan(&id, &name); err != nil {
				continue
			}
			invalidUsers = append(invalidUsers, fiber.Map{
				"id": id.String, 
				"name": name.String,
			})
		}

		// Delete invalid users
		result, err := db.Exec(`DELETE FROM emby_user WHERE id = '' OR id IS NULL`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		deleted, _ := result.RowsAffected()

		// Get final count
		var finalCount int
		db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&finalCount)

		return c.JSON(fiber.Map{
			"invalid_users_found": invalidUsers,
			"deleted_count": deleted,
			"final_total": finalCount,
		})
	}
}
