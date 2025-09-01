package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

type OverviewData struct {
	TotalUsers  int `json:"total_users"`
	TotalItems  int `json:"total_items"`
	TotalPlays  int `json:"total_plays"`
	UniquePlays int `json:"unique_plays"`
}

func Overview(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		data := OverviewData{}

		// Count users
		_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&data.TotalUsers)

		// Count library items (excluding live TV)
		_ = db.QueryRow(`SELECT COUNT(*) FROM library_item WHERE media_type NOT IN ('TvChannel', 'LiveTv', 'Channel')`).Scan(&data.TotalItems)

		// Count total play sessions (not events)
		_ = db.QueryRow(`SELECT COUNT(*) FROM play_sessions WHERE started_at IS NOT NULL`).Scan(&data.TotalPlays)

		// Count unique items played
		_ = db.QueryRow(`SELECT COUNT(DISTINCT item_id) FROM play_sessions WHERE started_at IS NOT NULL`).Scan(&data.UniquePlays)

		return c.JSON(data)
	}
}
