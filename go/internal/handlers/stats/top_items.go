package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type TopItem struct {
	ItemID string  `json:"item_id"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Hours  float64 `json:"hours"`
}

// /stats/top/items?days=30&limit=10
func TopItems(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		days := parseQueryInt(c, "days", 30)
		limit := parseQueryInt(c, "limit", 10)

		if days <= 0 {
			days = 30
		}
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		rows, err := db.Query(`
			SELECT li.id, li.name, li.type, COALESCE(SUM(pe.pos_ms), 0) / 3600000.0 AS hours
			FROM play_event pe
			JOIN library_item li ON li.id = pe.item_id
			WHERE pe.ts >= ?
			GROUP BY li.id, li.name, li.type
			ORDER BY hours DESC
			LIMIT ?;
		`, fromMs, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var out []TopItem
		for rows.Next() {
			var ti TopItem
			if err := rows.Scan(&ti.ItemID, &ti.Name, &ti.Type, &ti.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, ti)
		}
		return c.JSON(out)
	}
}
