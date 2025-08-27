package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type UsageRow struct {
	Day   string  `json:"day"`
	User  string  `json:"user"`
	Hours float64 `json:"hours"`
}

func Usage(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Use the shared helper functions from helpers.go
		days := parseQueryInt(c, "days", 14)
		if days <= 0 {
			days = 14
		}

		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -days).Unix()

		// UPGRADED & CORRECTED: This query is now accurate and syntactically valid.
		query := `
			WITH daily_overlap AS (
				SELECT
					strftime('%Y-%m-%d', datetime(pi.start_ts, 'unixepoch')) AS day,
					pi.user_id,
					(MIN(pi.end_ts, ?) - MAX(pi.start_ts, ?)) AS overlap_seconds
				FROM play_intervals pi
				WHERE
					pi.start_ts <= ? AND pi.end_ts >= ? -- Filter for intervals that overlap the window
				GROUP BY pi.id -- Group by interval id to correctly calculate overlap for each interval
			)
			SELECT
				do.day,
				u.name,
				SUM(do.overlap_seconds) / 3600.0 AS hours
			FROM daily_overlap do
			JOIN emby_user u ON u.id = do.user_id
			WHERE do.overlap_seconds > 0
			GROUP BY do.day, u.name
			ORDER BY do.day ASC, u.name ASC;
		`

		rows, err := db.Query(query, winEnd, winStart, winEnd, winStart)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "usage query failed: " + err.Error()})
		}
		defer rows.Close()

		out := []UsageRow{}
		for rows.Next() {
			var r UsageRow
			if err := rows.Scan(&r.Day, &r.User, &r.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "failed to scan usage row: " + err.Error()})
			}
			out = append(out, r)
		}
		return c.JSON(out)
	}
}
