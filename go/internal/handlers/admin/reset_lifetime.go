package admin

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v3"
)

// ResetLifetimeWatch recalculates lifetime watch data from play events
func ResetLifetimeWatch(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Clear existing lifetime_watch data
		_, err := db.Exec(`DELETE FROM lifetime_watch`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to clear lifetime data"})
		}

		// Recalculate based on play events with reasonable estimates
		rows, err := db.Query(`
			SELECT user_id, COUNT(*) as session_count, MAX(pos_ms) as max_pos
			FROM play_event 
			WHERE pos_ms > 30000
			GROUP BY user_id
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to query play events"})
		}
		defer rows.Close()

		updated := 0
		for rows.Next() {
			var userID string
			var sessionCount int
			var maxPos int64

			if err := rows.Scan(&userID, &sessionCount, &maxPos); err != nil {
				continue
			}

			// Estimate total watch time: sessions * average session time
			// Use conservative estimate: 45 minutes average session
			estimatedMs := int64(sessionCount) * 45 * 60 * 1000

			// Cap at reasonable maximum (max position seen)
			if estimatedMs > maxPos {
				estimatedMs = maxPos
			}

			_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms) VALUES (?, ?)`,
				userID, estimatedMs)
			if err == nil {
				updated++
			}
		}

		log.Printf("Reset lifetime watch data for %d users", updated)
		return c.JSON(fiber.Map{
			"success":       true,
			"users_updated": updated,
		})
	}
}
