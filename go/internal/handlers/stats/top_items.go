package stats

import (
	"database/sql"
	"log"
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
		// accept window=14d|4w, else days=30 as fallback
		days := parseWindowDays(c.Query("window", ""), parseQueryInt(c, "days", 30))
		limit := parseQueryInt(c, "limit", 10)

		if days <= 0 {
			days = 30
		}
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		// SMART APPROACH: Group by user+item+day to get unique viewing sessions
		// Then estimate time per session based on content type
		rows, err := db.Query(`
			SELECT 
				li.id, 
				COALESCE(li.name, 'Unknown') as name, 
				COALESCE(li.type, 'Unknown') as type,
				COUNT(DISTINCT pe.user_id || '-' || DATE(datetime(pe.ts / 1000, 'unixepoch'))) * 
				CASE 
					WHEN li.type = 'Movie' THEN 1.8
					WHEN li.type = 'Episode' THEN 0.7  
					ELSE 1.0
				END AS hours
			FROM play_event pe
			LEFT JOIN library_item li ON li.id = pe.item_id
			WHERE pe.ts >= ? AND pe.item_id != ''
			GROUP BY li.id, li.name, li.type
			ORDER BY hours DESC
			LIMIT ?;
		`, fromMs, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []TopItem{}
		for rows.Next() {
			var ti TopItem
			if err := rows.Scan(&ti.ItemID, &ti.Name, &ti.Type, &ti.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			log.Printf("Top item: id=%s, name='%s', type='%s', hours=%.2f",
				ti.ItemID, ti.Name, ti.Type, ti.Hours)
			out = append(out, ti)
		}
		return c.JSON(out)
	}
}
