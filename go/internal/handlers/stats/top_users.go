package stats

import (
	"database/sql"
	"time"

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

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		// Count unique viewing sessions (user+item+day combinations)
		rows, err := db.Query(`
			SELECT 
				u.id, 
				u.name,
				COUNT(DISTINCT pe.item_id || '-' || DATE(datetime(pe.ts / 1000, 'unixepoch'))) * 1.2 AS hours
			FROM play_event pe
			JOIN emby_user u ON u.id = pe.user_id
			WHERE pe.ts >= ? AND pe.user_id != ''
			GROUP BY u.id, u.name
			ORDER BY hours DESC
			LIMIT ?;
		`, fromMs, limit)
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
