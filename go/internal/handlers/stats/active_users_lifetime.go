package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

type ActiveUserLifetime struct {
	User    string `json:"user"`
	Days    int    `json:"days"`
	Hours   int    `json:"hours"`
	Minutes int    `json:"minutes"`
}

func ActiveUsersLifetime(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 5)
		if limit <= 0 || limit > 100 {
			limit = 5
		}

		rows, err := db.Query(`
			SELECT u.name, COALESCE(lw.total_ms,0)
			FROM lifetime_watch lw
			JOIN emby_user u ON u.id = lw.user_id
			ORDER BY lw.total_ms DESC
			LIMIT ?;
		`, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []ActiveUserLifetime{}
		for rows.Next() {
			var name string
			var totalMs int64
			if err := rows.Scan(&name, &totalMs); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Convert milliseconds to days/hours/minutes
			totalMinutes := int(totalMs / 60000)
			days := totalMinutes / (60 * 24)
			remainingMinutes := totalMinutes % (60 * 24)
			hours := remainingMinutes / 60
			minutes := remainingMinutes % 60

			out = append(out, ActiveUserLifetime{
				User:    name,
				Days:    days,
				Hours:   hours,
				Minutes: minutes,
			})
		}
		return c.JSON(out)
	}
}
