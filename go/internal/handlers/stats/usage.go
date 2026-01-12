package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/media"
)

type UsageRow struct {
	Day        string  `json:"day"`
	User       string  `json:"user"`
	ServerID   string  `json:"server_id"`
	ServerName string  `json:"server_name"`
	Hours      float64 `json:"hours"`
}

func Usage(db *sql.DB, mgr *media.MultiServerManager) fiber.Handler {
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
                u.server_id,
                SUM(
                    MAX(
                        0,
                        MIN(
                            MIN(pi.end_ts, ?) - MAX(pi.start_ts, ?),
                            CASE WHEN pi.duration_seconds IS NULL OR pi.duration_seconds <= 0
                                 THEN (pi.end_ts - pi.start_ts)
                                 ELSE pi.duration_seconds
                            END
                        )
                    )
                ) / 3600.0 AS hours
            FROM play_intervals pi
            JOIN emby_user u ON u.id = pi.user_id AND u.deleted_at IS NULL
            LEFT JOIN library_item li ON li.id = pi.item_id
            WHERE
                pi.start_ts <= ? AND pi.end_ts >= ?
                AND COALESCE(li.media_type, 'Unknown') NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')
            GROUP BY day, u.name, u.server_id
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
			if err := rows.Scan(&r.Day, &r.User, &r.ServerID, &r.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "failed to scan usage row: " + err.Error()})
			}
		    out = append(out, r)
		}
		
		// Fill in server names
		configs := mgr.GetServerConfigs()
		for i := range out {
			if cfg, ok := configs[out[i].ServerID]; ok {
				out[i].ServerName = cfg.Name
			} else {
				out[i].ServerName = out[i].ServerID
			}
		}

		return c.JSON(out)
	}
}
