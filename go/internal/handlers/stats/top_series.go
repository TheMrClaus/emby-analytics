package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type TopSeriesRow struct {
	SeriesID string  `json:"series_id"`
	Name     string  `json:"name"`
	Hours    float64 `json:"hours"`
}

// TopSeries aggregates watch time per series across episodes within a time window.
func TopSeries(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		timeframe := c.Query("timeframe", "")
		if timeframe == "" {
			days := parseQueryInt(c, "days", 14)
			switch {
			case days <= 0:
				timeframe = "all-time"
			case days == 1:
				timeframe = "1d"
			case days == 3:
				timeframe = "3d"
			case days == 7:
				timeframe = "7d"
			case days == 14:
				timeframe = "14d"
			case days == 30:
				timeframe = "30d"
			default:
				timeframe = "30d"
			}
		}
		limit := parseQueryInt(c, "limit", 10)
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -parseTimeframeToDays(timeframe)).Unix()
		if timeframe == "all-time" {
			winStart = 0
			winEnd = now.AddDate(100, 0, 0).Unix()
		}

		// Prefer series_id grouping when available, otherwise group by derived series name.
		// Sum overlap within window using MIN/MAX clamp.
		rows, err := db.Query(`
            WITH iv AS (
                SELECT 
                    COALESCE(li.series_id, '') AS sid,
                    COALESCE(li.series_name, CASE WHEN INSTR(li.name,' - ')>0 THEN SUBSTR(li.name,1,INSTR(li.name,' - ')-1) ELSE li.name END) AS sname,
                    MIN(pi.end_ts, ?) - MAX(pi.start_ts, ?) AS overlap
                FROM play_intervals pi
                JOIN library_item li ON li.id = pi.item_id
                WHERE li.media_type='Episode' AND `+excludeLiveTvFilter()+`
                  AND pi.start_ts <= ? AND pi.end_ts >= ?
                GROUP BY pi.id
            )
            SELECT sid, sname, SUM(CASE WHEN overlap>0 THEN overlap ELSE 0 END) / 3600.0 AS hours
            FROM iv
            WHERE sname IS NOT NULL AND sname <> ''
            GROUP BY sid, sname
            ORDER BY hours DESC
            LIMIT ?
        `, winEnd, winStart, winEnd, winStart, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []TopSeriesRow{}
		for rows.Next() {
			var sid, name string
			var hrs float64
			if err := rows.Scan(&sid, &name, &hrs); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, TopSeriesRow{SeriesID: sid, Name: name, Hours: hrs})
		}
		return c.JSON(out)
	}
}
