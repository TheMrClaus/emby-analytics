package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type UsagePoint struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
}

func Usage(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		rows, err := db.Query(`
			SELECT
				strftime('%Y-%m-%d', datetime(ts / 1000, 'unixepoch')) as day,
				SUM(pos_ms) / 1000.0 / 60.0 / 60.0 as hours
			FROM play_event
			GROUP BY day
			ORDER BY day ASC
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var result []UsagePoint
		for rows.Next() {
			var day string
			var hours float64
			if err := rows.Scan(&day, &hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			result = append(result, UsagePoint{
				Date:  day,
				Hours: hours,
			})
		}

		// Fill in missing dates
		if len(result) > 0 {
			var filled []UsagePoint
			start, _ := time.Parse("2006-01-02", result[0].Date)
			end, _ := time.Parse("2006-01-02", result[len(result)-1].Date)
			dateMap := make(map[string]float64)
			for _, p := range result {
				dateMap[p.Date] = p.Hours
			}
			for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
				ds := d.Format("2006-01-02")
				filled = append(filled, UsagePoint{
					Date:  ds,
					Hours: dateMap[ds],
				})
			}
			result = filled
		}

		return c.JSON(result)
	}
}
