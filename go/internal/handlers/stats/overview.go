package stats

import (
	"database/sql"
	"log"
	"time"

	"emby-analytics/internal/handlers/admin"

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
		start := time.Now()
		data := OverviewData{}

		// Count users
		err := db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&data.TotalUsers)
		if err != nil {
			log.Printf("[overview] Error counting users: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to count users"})
		}

		// Count unique library items (excluding live TV, deduplicated by normalized file_path across servers)
		// Only count items that have file_path populated (excludes servers without path support)
		// Normalize paths by extracting everything after common library folders (Movies/, TV/, etc)
		err = db.QueryRow(`
			SELECT COUNT(DISTINCT
				COALESCE(
					NULLIF(
						CASE WHEN INSTR(LOWER(REPLACE(file_path, '\', '/')), '/movies/') > 0
							THEN SUBSTR(file_path, INSTR(LOWER(REPLACE(file_path, '\', '/')), '/movies/') + LENGTH('/movies/'))
							ELSE NULL END, 
						''
					),
					NULLIF(
						CASE WHEN INSTR(LOWER(REPLACE(file_path, '\', '/')), '/tv/') > 0
							THEN SUBSTR(file_path, INSTR(LOWER(REPLACE(file_path, '\', '/')), '/tv/') + LENGTH('/tv/'))
							ELSE NULL END,
						''
					),
					NULLIF(
						CASE WHEN INSTR(LOWER(REPLACE(file_path, '\', '/')), '/shows/') > 0
							THEN SUBSTR(file_path, INSTR(LOWER(REPLACE(file_path, '\', '/')), '/shows/') + LENGTH('/shows/'))
							ELSE NULL END,
						''
					),
					file_path
				)
			)
			FROM library_item
			WHERE media_type NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')
				AND file_path IS NOT NULL
				AND file_path != ''
		`).Scan(&data.TotalItems)
		if err != nil {
			log.Printf("[overview] Error counting library items: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to count library items"})
		}

		// Count total play sessions (exclude Live TV)
		err = db.QueryRow(`SELECT COUNT(*) FROM play_sessions WHERE started_at IS NOT NULL AND COALESCE(item_type,'') NOT IN ('TvChannel','LiveTv','Channel','TvProgram')`).Scan(&data.TotalPlays)
		if err != nil {
			log.Printf("[overview] Error counting play sessions: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to count play sessions"})
		}

		// Count unique items played (exclude Live TV)
		err = db.QueryRow(`SELECT COUNT(DISTINCT item_id) FROM play_sessions WHERE started_at IS NOT NULL AND COALESCE(item_type,'') NOT IN ('TvChannel','LiveTv','Channel','TvProgram')`).Scan(&data.UniquePlays)
		if err != nil {
			log.Printf("[overview] Error counting unique plays: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to count unique plays"})
		}

		duration := time.Since(start)
		isSlowQuery := duration > 1*time.Second
		if isSlowQuery {
			log.Printf("[overview] WARNING: Slow query took %v", duration)
		}

		// Track metrics
		admin.IncrementQueryMetrics(duration, isSlowQuery)

		log.Printf("[overview] Successfully fetched data in %v: users=%d, items=%d, plays=%d, unique=%d",
			duration, data.TotalUsers, data.TotalItems, data.TotalPlays, data.UniquePlays)

		return c.JSON(data)
	}
}
