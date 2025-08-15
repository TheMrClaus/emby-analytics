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
		days := parseWindowDays(c.Query("window", ""), parseQueryInt(c, "days", 30))
		if days <= 0 {
			days = 30
		}
		limit := parseQueryInt(c, "limit", 0) // 0 = no limit

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		q := `
			SELECT li.codec, COUNT(DISTINCT li.id) as count
			FROM play_event pe
			JOIN library_item li ON li.id = pe.item_id
			WHERE li.codec IS NOT NULL
			  AND pe.ts >= ?
			GROUP BY li.codec
			ORDER BY count DESC
		`
		var rows *sql.Rows
		var err error
		if limit > 0 && limit <= 100 {
			q = q + " LIMIT ?"
			rows, err = db.Query(q, fromMs, limit)
		} else {
			rows, err = db.Query(q, fromMs)
		}
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
