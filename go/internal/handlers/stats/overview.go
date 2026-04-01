package stats

import (
	"database/sql"
	"fmt"
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

		// Count users (exclude soft-deleted users)
		err := db.QueryRow(`SELECT COUNT(*) FROM emby_user WHERE deleted_at IS NULL`).Scan(&data.TotalUsers)
		if err != nil {
			log.Printf("[overview] Error counting users: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to count users"})
		}

		// Count unique library items using normalized paths and including pathless items
		// Uses the same normalization as other stats endpoints for consistency
		normalizedPath := normalizedFilePathExpr("")
		query := fmt.Sprintf(`
			SELECT COUNT(*) FROM (
				-- Items with file paths: dedupe by normalized path
				SELECT DISTINCT 'path:' || (%s) AS dedupe_key
				FROM library_item
				WHERE media_type NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')
					AND file_path IS NOT NULL
					AND TRIM(file_path) != ''
				UNION
				-- Items without file paths: count by ID (no cross-server deduplication possible)
				SELECT DISTINCT 'id:' || id AS dedupe_key
				FROM library_item
				WHERE media_type NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')
					AND (file_path IS NULL OR TRIM(file_path) = '')
			)
		`, normalizedPath)
		
		err = db.QueryRow(query).Scan(&data.TotalItems)
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
