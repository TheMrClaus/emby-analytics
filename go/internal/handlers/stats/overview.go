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
		var data OverviewData

		// Count total users
		err := db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&data.TotalUsers)
		if err != nil && err != sql.ErrNoRows {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Count total items (library_item table in schema)
		err = db.QueryRow(`SELECT COUNT(*) FROM library_item`).Scan(&data.TotalItems)
		if err != nil && err != sql.ErrNoRows {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Count total plays (events)
		err = db.QueryRow(`SELECT COUNT(*) FROM play_event`).Scan(&data.TotalPlays)
		if err != nil && err != sql.ErrNoRows {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Count unique played items
		err = db.QueryRow(`SELECT COUNT(DISTINCT item_id) FROM play_event`).Scan(&data.UniquePlays)
		if err != nil && err != sql.ErrNoRows {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(data)
	}
}
