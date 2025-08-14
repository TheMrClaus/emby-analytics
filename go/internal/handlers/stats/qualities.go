package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type QualityStat struct {
	Height int `json:"height"`
	Count  int `json:"count"`
}

func Qualities(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		days := parseQueryInt(c, "days", 30)
		if days <= 0 {
			days = 30
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		rows, err := db.Query(`
			SELECT li.height, COUNT(DISTINCT li.id) as count
			FROM play_event pe
			JOIN library_item li ON li.id = pe.item_id
			WHERE li.height IS NOT NULL
			  AND pe.ts >= ?
			GROUP BY li.height
			ORDER BY li.height DESC;
		`, fromMs)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []QualityStat{}
		for rows.Next() {
			var q QualityStat
			if err := rows.Scan(&q.Height, &q.Count); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, q)
		}
		return c.JSON(out)
	}
}
