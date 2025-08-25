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
		db.QueryRow(`SELECT COUNT(*) FROM play_event`).Scan(&totalEvents)
		db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM play_event WHERE pos_ms > 30000`).Scan(&totalUsers)

		log.Printf("DEBUG: Found %d total play events, %d users with >30s position", totalEvents, totalUsers)

		if totalEvents == 0 {
			return c.JSON(fiber.Map{
				"success":           true,
				"users_updated":     0,
				"total_watch_hours": 0,
				"calculation_type":  "actual_duration",
				"timestamp":         time.Now().Unix(),
				"debug_info":        "No play events found in database",
			})
		}

		// Use a simpler approach for now - calculate watch time based on position progression
		rows, err := db.Query(`
			SELECT 
				pe1.user_id,
				SUM(CASE 
					WHEN pe2.pos_ms > pe1.pos_ms 
						AND pe2.pos_ms - pe1.pos_ms <= 600000  -- Max 10 min jump
						AND pe2.ts - pe1.ts <= 1800000         -- Max 30 min time gap
					THEN pe2.pos_ms - pe1.pos_ms
					ELSE 0
				END) as calculated_watch_ms,
				COUNT(*) as total_events,
				MAX(pe1.pos_ms) as max_position_ms
			FROM play_event pe1
			LEFT JOIN play_event pe2 ON pe2.user_id = pe1.user_id 
				AND pe2.item_id = pe1.item_id 
				AND pe2.ts > pe1.ts
				AND pe2.ts = (
					SELECT MIN(pe3.ts) 
					FROM play_event pe3 
					WHERE pe3.user_id = pe1.user_id 
						AND pe3.item_id = pe1.item_id 
						AND pe3.ts > pe1.ts
				)
			WHERE pe1.pos_ms > 30000  -- Only events after 30 seconds
			GROUP BY pe1.user_id
			HAVING calculated_watch_ms > 60000  -- At least 1 minute total
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate durations: " + err.Error()})
		}
		defer rows.Close()

		updated := 0
		totalWatchHours := float64(0)

		for rows.Next() {
			var userID string
			var calculatedWatchMs, totalEvents, maxPositionMs int64

			if err := rows.Scan(&userID, &calculatedWatchMs, &totalEvents, &maxPositionMs); err != nil {
				log.Printf("Error scanning user data: %v", err)
				continue
			}

			// Use the larger of calculated watch time or a conservative estimate
			finalWatchMs := calculatedWatchMs
			if finalWatchMs == 0 {
				// Fallback: use max position as watch time (conservative)
				finalWatchMs = maxPositionMs
			}

			if finalWatchMs > 0 {
				// Insert the calculated watch time
				_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms) VALUES (?, ?)`,
					userID, finalWatchMs)

				if err == nil {
					updated++
					watchHours := float64(finalWatchMs) / (1000.0 * 60.0 * 60.0)
					totalWatchHours += watchHours

					// Log detailed info for verification
					log.Printf("User %s: %.1f hours watch time (%d events, max pos %.1f hrs)",
						userID,
						watchHours,
						totalEvents,
						float64(maxPositionMs)/(1000.0*60.0*60.0))
				} else {
					log.Printf("Error inserting lifetime data for user %s: %v", userID, err)
				}
			}
		}

		log.Printf("✓ Calculated watch durations for %d users (%.1f total hours)",
			updated, totalWatchHours)

		return c.JSON(fiber.Map{
			"success":           true,
			"users_updated":     updated,
			"total_watch_hours": totalWatchHours,
			"calculation_type":  "simplified_actual_duration",
			"timestamp":         time.Now().Unix(),
			"debug_info":        fmt.Sprintf("%d total events, %d qualifying users", totalEvents, totalUsers),
		})
	}
}
