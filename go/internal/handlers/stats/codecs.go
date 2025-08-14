package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type CodecStat struct {
	Codec string `json:"codec"`
	Count int    `json:"count"`
}

func Codecs(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		days := parseQueryInt(c, "days", 30)
		if days <= 0 {
			days = 30
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		rows, err := db.Query(`
			SELECT li.codec, COUNT(DISTINCT li.id) as count
			FROM play_event pe
			JOIN library_item li ON li.id = pe.item_id
			WHERE li.codec IS NOT NULL
			  AND pe.ts >= ?
			GROUP BY li.codec
			ORDER BY count DESC;
		`, fromMs)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []CodecStat{}
		for rows.Next() {
			var cs CodecStat
			if err := rows.Scan(&cs.Codec, &cs.Count); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, cs)
		}
		return c.JSON(out)
	}
}
