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

// GET /stats/usage?days=14  (also supports window=14d/4w)
func Usage(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// support either ?window=14d or ?days=14
		days := parseWindowDays(c.Query("window", ""), parseQueryInt(c, "days", 14))
		if days <= 0 {
			days = 14
		}
		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		// Use simple daily usage approximation based on lifetime totals
		rows, err := db.Query(`
			SELECT
				strftime('%Y-%m-%d', datetime(pe.ts / 1000, 'unixepoch')) AS day,
				COALESCE(u.name, pe.user_id) AS user,
				COUNT(DISTINCT pe.item_id) * 0.6 AS hours  -- Conservative daily estimate
			FROM play_event pe
			LEFT JOIN emby_user u ON u.id = pe.user_id
			WHERE pe.ts >= ? AND pe.user_id != '' AND pe.pos_ms > 120000  -- 2+ minutes
			GROUP BY day, pe.user_id, u.name
			ORDER BY day ASC, user ASC;
		`, fromMs)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []UsageRow{}
		for rows.Next() {
			var r UsageRow
			if err := rows.Scan(&r.Day, &r.User, &r.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, r)
		}
		return c.JSON(out)
	}
}
