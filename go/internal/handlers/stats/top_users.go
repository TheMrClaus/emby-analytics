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
		// Get timeframe parameter: all-time, 30d, 14d, 7d, 3d, 1d
		timeframe := c.Query("timeframe", "14d")
		limit := parseQueryInt(c, "limit", 10)

		if limit <= 0 || limit > 100 {
			limit = 10
		}

		// Handle "all-time" differently - use lifetime_watch for perfect accuracy
		if timeframe == "all-time" {
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

		// Handle time-windowed queries - parse days from timeframe
		days := parseTimeframeToDays(timeframe)
		if days <= 0 {
			days = 14 // fallback
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		// First try the accurate position-based calculation with time window
		rows, err := db.Query(`
			SELECT
				u.id,
				u.name,
				SUM(max_pos_ms) / 3600000.0 AS hours
			FROM (
				-- Get the max watch position for each item within the time window
				SELECT
					user_id,
					item_id,
					MAX(pos_ms) as max_pos_ms
				FROM play_event
				WHERE ts >= ? AND user_id != '' AND pos_ms > 60000  -- 1+ minute sessions
				GROUP BY user_id, item_id
			) AS user_item_max
			JOIN emby_user u ON u.id = user_item_max.user_id
			GROUP BY u.id, u.name
			HAVING SUM(max_pos_ms) > 600000  -- At least 10 minutes total
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

		// If we have no data from play_event table, fall back to lifetime_watch
		// This provides some data until play_event table rebuilds
		if len(out) == 0 {
			rows2, err2 := db.Query(`
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
			if err2 == nil {
				defer rows2.Close()
				for rows2.Next() {
					var u TopUser
					if err := rows2.Scan(&u.UserID, &u.Name, &u.Hours); err == nil {
						out = append(out, u)
					}
				}
			}
		}

		return c.JSON(out)
	}
}

// parseTimeframeToDays converts timeframe strings to days
func parseTimeframeToDays(timeframe string) int {
	switch timeframe {
	case "1d":
		return 1
	case "3d":
		return 3
	case "7d":
		return 7
	case "14d":
		return 14
	case "30d":
		return 30
	case "all-time":
		return 0 // Special case handled separately
	default:
		return 14 // Default fallback
	}
}
