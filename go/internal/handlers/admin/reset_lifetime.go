package admin

import (
	"database/sql"
	"fmt"
	"log"
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

		// First, let's debug what data we have
		var totalEvents, totalUsers int
		_ = db.QueryRow(`SELECT COUNT(*) FROM play_event`).Scan(&totalEvents)
		_ = db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM play_event WHERE pos_ms > 30000`).Scan(&totalUsers)

		log.Printf("DEBUG: Found %d total play events, %d users with >30s position", totalEvents, totalUsers)

		if totalEvents == 0 {
			return c.JSON(fiber.Map{
				"success":           true,
				"users_updated":     0,
				"total_watch_hours": 0,
				"calculation_type":  "max_position_per_item",
				"timestamp":         time.Now().Unix(),
				"debug_info":        "No play events found in database",
			})
		}

		// Use the simplest and most accurate approach: max position per user per item
		rows, err := db.Query(`
			SELECT 
				user_id,
				SUM(max_pos_ms) AS total_watch_ms,
				COUNT(DISTINCT item_id) AS items_watched,
				AVG(max_pos_ms) AS avg_item_watch_ms
			FROM (
				-- Get the maximum position reached per user per item
				SELECT 
					user_id, 
					item_id, 
					MAX(pos_ms) AS max_pos_ms
				FROM play_event 
				WHERE pos_ms > 30000  -- Only events after 30 seconds
				GROUP BY user_id, item_id
			) user_item_max
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
			var totalWatchMs int64
			var itemsWatched int64
			var avgItemWatchMs float64

			if err := rows.Scan(&userID, &totalWatchMs, &itemsWatched, &avgItemWatchMs); err != nil {

				log.Printf("Error scanning user data: %v", err)
				continue
			}

			// Insert the calculated watch time
			_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms) VALUES (?, ?)`,
				userID, totalWatchMs)
			if err != nil {
				log.Printf("Error inserting lifetime data for user %s: %v", userID, err)
				continue
			}

			updated++
			watchHours := float64(totalWatchMs) / (1000.0 * 60.0 * 60.0)
			totalWatchHours += watchHours

			// Log detailed info for verification
			log.Printf("User %s: %.1f hours actual watch time (%d items, avg %.1f min per item)",
				userID,
				watchHours,
				itemsWatched,
				avgItemWatchMs/(1000.0*60.0),
			)
		}

		if err := rows.Err(); err != nil {
			log.Printf("Row iteration error: %v", err)
		}

		log.Printf("✓ Calculated watch durations for %d users (%.1f total hours)",
			updated, totalWatchHours)

		return c.JSON(fiber.Map{
			"success":           true,
			"users_updated":     updated,
			"total_watch_hours": totalWatchHours,
			"calculation_type":  "max_position_per_item",
			"timestamp":         time.Now().Unix(),
			"debug_info":        fmt.Sprintf("%d total events, %d qualifying users", totalEvents, totalUsers),
		})
	}
}
