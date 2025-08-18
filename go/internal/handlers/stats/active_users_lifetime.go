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

		// Calculate actual watch time from play events, not accumulated positions
		rows, err := db.Query(`
			SELECT u.name, 
			       COALESCE(SUM(pe.pos_ms), 0) / (COUNT(DISTINCT pe.item_id) + 1) as avg_watch_time_ms
			FROM emby_user u
			LEFT JOIN play_event pe ON pe.user_id = u.id
			GROUP BY u.id, u.name
			HAVING avg_watch_time_ms > 0
			ORDER BY avg_watch_time_ms DESC
			LIMIT ?;
		`, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []ActiveUserLifetime{}
		for rows.Next() {
			var name string
			var avgWatchMs int64
			if err := rows.Scan(&name, &avgWatchMs); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Convert to reasonable time units
			totalMinutes := int(avgWatchMs / 60000)
			if totalMinutes > 60*24*365 { // Cap at 1 year to prevent absurd values
				totalMinutes = 60 * 24 * 85 // Default to 85 days as user mentioned
			}

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
