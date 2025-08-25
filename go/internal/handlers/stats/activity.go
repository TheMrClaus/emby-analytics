package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type ActivityEntry struct {
	Timestamp int64   `json:"timestamp"`
	UserID    string  `json:"user_id"`
	UserName  string  `json:"user_name"`
	ItemID    string  `json:"item_id"`
	ItemName  string  `json:"item_name"`
	ItemType  string  `json:"item_type"`
	PosHours  float64 `json:"pos_hours"`
}

// /stats/activity?days=7&limit=20
func Activity(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		days := parseQueryInt(c, "days", 7)
		limit := parseQueryInt(c, "limit", 20)

		if days <= 0 {
			days = 7
		}
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		rows, err := db.Query(`
			SELECT pe.ts, u.id, u.name, li.id, li.name, li.type, pe.pos_ms / 3600000.0
			FROM play_event pe
			LEFT JOIN emby_user u ON u.id = pe.user_id
			LEFT JOIN library_item li ON li.id = pe.item_id
			WHERE pe.ts >= ? AND li.type NOT IN ('TvChannel', 'LiveTv', 'Channel')
			ORDER BY pe.ts DESC
			LIMIT ?;
		`, fromMs, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := []ActivityEntry{}
		for rows.Next() {
			var a ActivityEntry
			if err := rows.Scan(&a.Timestamp, &a.UserID, &a.UserName, &a.ItemID, &a.ItemName, &a.ItemType, &a.PosHours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, a)
		}
		return c.JSON(out)
	}
}
