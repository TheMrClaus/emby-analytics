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

		// Count total users
		_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&data.TotalUsers)

		// Count total items (excluding Live TV)
		_ = db.QueryRow(`SELECT COUNT(*) FROM library_item WHERE type NOT IN ('TvChannel', 'LiveTv', 'Channel')`).Scan(&data.TotalItems)

		// Count total plays (events)
		_ = db.QueryRow(`SELECT COUNT(*) FROM play_event`).Scan(&data.TotalPlays)

		// Count unique played items
		_ = db.QueryRow(`SELECT COUNT(DISTINCT item_id) FROM play_event`).Scan(&data.UniquePlays)

		return c.JSON(data)
	}
}
