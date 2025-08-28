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
		days := parseQueryInt(c, "days", 14)
		if days <= 0 {
			days = 14
		}

		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -days).Unix()

		// CORRECTED & SIMPLIFIED: This query correctly calculates the overlap
		// duration for each interval within the window and then sums it up per day and user.
		query := `
			SELECT
				strftime('%Y-%m-%d', datetime(pi.start_ts, 'unixepoch')) AS day,
				u.name,
				SUM(
					-- Calculate the overlap of each interval with the time window
					MIN(pi.end_ts, ?) - MAX(pi.start_ts, ?)
				) / 3600.0 AS hours
			FROM play_intervals pi
			JOIN emby_user u ON u.id = pi.user_id
			WHERE
				pi.start_ts <= ? AND pi.end_ts >= ? -- Filter for intervals that overlap the window
			GROUP BY day, u.name
			ORDER BY day ASC, u.name ASC;
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
