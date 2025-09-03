package stats

import (
	"database/sql"
	"emby-analytics/internal/handlers/settings"

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

		// Get the setting for whether to include Trakt items
		includeTrakt := settings.GetSettingBool(db, "include_trakt_items", false)

		rows, err := db.Query(`
			SELECT 
				u.name,
				COALESCE(lw.emby_ms, 0) AS emby_ms,
				COALESCE(lw.trakt_ms, 0) AS trakt_ms
			FROM lifetime_watch lw
			JOIN emby_user u ON u.id = lw.user_id
			WHERE lw.emby_ms > 0 OR lw.trakt_ms > 0
			ORDER BY 
				CASE WHEN ? = 1 THEN (COALESCE(lw.emby_ms, 0) + COALESCE(lw.trakt_ms, 0))
				     ELSE COALESCE(lw.emby_ms, 0) END DESC
			LIMIT ?;
		`, includeTrakt, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []ActiveUserLifetime{}
		for rows.Next() {
			var name string
			var embyMs, traktMs int64
			if err := rows.Scan(&name, &embyMs, &traktMs); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Calculate total based on setting
			totalMs := embyMs
			if includeTrakt {
				totalMs += traktMs
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
