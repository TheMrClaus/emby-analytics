package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

type AccurateTopUser struct {
	UserID      string  `json:"user_id"`
	Name        string  `json:"name"`
	Hours       float64 `json:"hours"`
	Days        int     `json:"days"`
	PlayedItems int     `json:"played_items"`
}

// AccurateTopUsers uses the true Emby approach: sum of full runtime of "Played" items
func AccurateTopUsers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 10)

		if limit <= 0 || limit > 100 {
			limit = 10
		}

		// Use the accurate lifetime watch data calculated by user sync
		// This data comes from Emby's "Played" flag + full RunTimeTicks
		rows, err := db.Query(`
			SELECT 
				u.id, 
				u.name,
				COALESCE(lw.total_ms / 3600000.0, 0) AS hours,
				COALESCE(lw.total_ms / (24 * 3600000), 0) AS days,
				-- Estimate played items based on average content length
				COALESCE(lw.total_ms / (90 * 60 * 1000), 0) AS played_items
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

		out := []AccurateTopUser{}
		for rows.Next() {
			var u AccurateTopUser
			var daysFloat float64
			var playedFloat float64
			if err := rows.Scan(&u.UserID, &u.Name, &u.Hours, &daysFloat, &playedFloat); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			u.Days = int(daysFloat)
			u.PlayedItems = int(playedFloat)
			out = append(out, u)
		}
		return c.JSON(out)
	}
}
