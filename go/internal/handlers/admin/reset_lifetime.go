package admin

import (
	"emby-analytics/internal/logging"
	"database/sql"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

// ResetLifetimeWatch recalculates lifetime watch data from actual play event durations
func ResetLifetimeWatch(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Clear existing lifetime_watch data
		_, err := db.Exec(`DELETE FROM lifetime_watch`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to clear lifetime data"})
		}

		// First, let's debug what data we have - updated table name
		var totalEvents, totalUsers int
		_ = db.QueryRow(`SELECT COUNT(*) FROM play_intervals`).Scan(&totalEvents)
		_ = db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM play_intervals WHERE duration_seconds > 30`).Scan(&totalUsers)

		logging.Debug("DEBUG: Found %d total play intervals, %d users with >30s duration", totalEvents, totalUsers)

		if totalEvents == 0 {
			return c.JSON(fiber.Map{
				"success":           true,
				"users_updated":     0,
				"total_watch_hours": 0,
				"calculation_type":  "play_intervals",
				"timestamp":         time.Now().Unix(),
				"debug_info":        "No play intervals found in database",
			})
		}

		// Use play_intervals table for more accurate calculation
		rows, err := db.Query(`
			SELECT 
				user_id,
				SUM(duration_seconds * 1000) AS total_watch_ms,
				COUNT(DISTINCT item_id) AS items_watched,
				AVG(duration_seconds * 1000) AS avg_item_watch_ms
			FROM play_intervals 
			WHERE duration_seconds > 30  -- Only intervals longer than 30 seconds
			GROUP BY user_id
			HAVING total_watch_ms > 60000  -- At least 1 minute total
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate durations: " + err.Error()})
		}
		defer rows.Close()

		updated := 0
		totalWatchHours := float64(0)

		for rows.Next() {
			var userID string
			var totalWatchMs, itemsWatched int64
			var avgItemWatchMs float64

			if err := rows.Scan(&userID, &totalWatchMs, &itemsWatched, &avgItemWatchMs); err != nil {
				logging.Debug("Error scanning user data: %v", err)
				continue
			}

			// Insert the calculated watch time
			_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms) VALUES (?, ?)`,
				userID, totalWatchMs)
			if err != nil {
				logging.Debug("Error inserting lifetime data for user %s: %v", userID, err)
				continue
			}

			updated++
			totalWatchHours += float64(totalWatchMs) / (1000.0 * 60.0 * 60.0)

			logging.Debug("User %s: %d items watched, %.1f hours total (avg %.1f min/item)",
				userID, itemsWatched, float64(totalWatchMs)/(1000.0*60.0*60.0), avgItemWatchMs/(1000.0*60.0))
		}

		return c.JSON(fiber.Map{
			"success":           true,
			"users_updated":     updated,
			"total_watch_hours": fmt.Sprintf("%.1f", totalWatchHours),
			"calculation_type":  "play_intervals",
			"timestamp":         time.Now().Unix(),
		})
	}
}
