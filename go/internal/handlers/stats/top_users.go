package stats

import (
	"database/sql"
	"emby-analytics/internal/queries"
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
		timeframe := c.Query("timeframe", "14d")
		limit := parseQueryInt(c, "limit", 10)

		if limit <= 0 || limit > 100 {
			limit = 10
		}

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

		days := parseTimeframeToDays(timeframe)
		if days <= 0 {
			days = 14
		}

		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -days).Unix()

		// CORRECTED: Pass 'c' directly as the context.
		queryRows, err := queries.TopUsersByWatchSeconds(c, db, winStart, winEnd, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		out := make([]TopUser, len(queryRows))
		for i, r := range queryRows {
			out[i] = TopUser{
				UserID: r.UserID,
				Name:   r.Name,
				Hours:  r.Hours,
			}
		}

		return c.JSON(out)
	}
}
