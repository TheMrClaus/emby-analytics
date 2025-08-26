package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

type TopUser struct {
	UserID string  `json:"user_id"`
	Name   string  `json:"name"`
	Hours  float64 `json:"hours"`
}

func TopUsers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// accept window=14d|4w, else days=30 as fallback
		days := parseWindowDays(c.Query("window", ""), parseQueryInt(c, "days", 30))
		limit := parseQueryInt(c, "limit", 10)

		if days <= 0 {
			days = 30
		}
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		// Use accurate lifetime watch data (Emby's "Played" flag + full runtime)
		rows, err := db.Query(`
			SELECT 
				u.id, 
				u.name,
				COALESCE(lw.total_ms / 3600000.0, 0) AS hours
			FROM emby_user u
			LEFT JOIN lifetime_watch lw ON lw.user_id = u.id
			WHERE lw.total_ms > 0
			ORDER BY hours DESC
			LIMIT ?;
		`, limit)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []TopUser{}
		for rows.Next() {
			var u TopUser
			if err := rows.Scan(&u.UserID, &u.Name, &u.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, u)
		}
		return c.JSON(out)
	}
}
