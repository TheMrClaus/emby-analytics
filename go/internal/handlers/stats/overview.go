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
		data := OverviewData{} // ensure all fields start at 0

		// This query is correct and does not need to change.
		_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&data.TotalUsers)

		// This query is also correct.
		_ = db.QueryRow(`SELECT COUNT(*) FROM library_item WHERE media_type NOT IN ('TvChannel', 'LiveTv', 'Channel')`).Scan(&data.TotalItems)

		// CORRECTED: Count sessions instead of old play events.
		_ = db.QueryRow(`SELECT COUNT(*) FROM play_sessions`).Scan(&data.TotalPlays)

		// CORRECTED: Count unique items from sessions.
		_ = db.QueryRow(`SELECT COUNT(DISTINCT item_id) FROM play_sessions`).Scan(&data.UniquePlays)

		return c.JSON(data)
	}
}
