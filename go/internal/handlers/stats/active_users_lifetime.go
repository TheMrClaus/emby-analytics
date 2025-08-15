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

// GET /stats/active-users-lifetime?limit=1
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
			mins := int(totalMs / 60000)
			days := mins / (60 * 24)
			rem := mins % (60 * 24)
			hrs := rem / 60
			m := rem % 60
			out = append(out, ActiveUserLifetime{
				User:    name,
				Days:    days,
				Hours:   hrs,
				Minutes: m,
			})
		}
		return c.JSON(out)
	}
}
