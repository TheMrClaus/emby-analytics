package admin

import (
	"database/sql"
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

		// Calculate actual watch durations by analyzing position progression
		rows, err := db.Query(`
			WITH user_item_sessions AS (
				-- Group events by user, item, and session (gaps > 5 minutes = new session)
				SELECT 
					user_id,
					item_id,
					ts,
					pos_ms,
					LAG(ts) OVER (PARTITION BY user_id, item_id ORDER BY ts) as prev_ts,
					LAG(pos_ms) OVER (PARTITION BY user_id, item_id ORDER BY ts) as prev_pos_ms,
					CASE WHEN ts - LAG(ts) OVER (PARTITION BY user_id, item_id ORDER BY ts) > 300000 -- 5 min gap
						THEN 1 ELSE 0 END as session_start
				FROM play_event 
				WHERE pos_ms > 30000  -- Only events after 30 seconds
				ORDER BY user_id, item_id, ts
			),
			session_groups AS (
				-- Assign session numbers based on gaps
				SELECT 
					user_id,
					item_id,
					ts,
					pos_ms,
					prev_ts,
					prev_pos_ms,
					SUM(session_start) OVER (
						PARTITION BY user_id, item_id 
						ORDER BY ts 
						ROWS UNBOUNDED PRECEDING
					) as session_id
				FROM user_item_sessions
			),
			session_durations AS (
				-- Calculate actual watch time per session based on position advancement
				SELECT 
					user_id,
					item_id,
					session_id,
					MIN(ts) as session_start,
					MAX(ts) as session_end,
					MIN(pos_ms) as start_pos,
					MAX(pos_ms) as end_pos,
					-- Calculate duration as position advancement (more accurate than time-based)
					-- But cap individual jumps at 10 minutes to handle seeks/scrubbing
					SUM(CASE 
						WHEN prev_pos_ms IS NOT NULL 
							AND pos_ms > prev_pos_ms 
							AND (pos_ms - prev_pos_ms) <= 600000 -- Max 10 min jump
							AND (ts - prev_ts) <= 900000 -- Max 15 min time gap within session
						THEN pos_ms - prev_pos_ms
						ELSE 0
					END) as actual_watch_ms
				FROM session_groups
				GROUP BY user_id, item_id, session_id
			)
			-- Aggregate all sessions per user
			SELECT 
				user_id,
				SUM(actual_watch_ms) as total_actual_ms,
				COUNT(DISTINCT item_id) as items_watched,
				COUNT(DISTINCT session_id) as total_sessions,
				AVG(actual_watch_ms) as avg_session_ms
			FROM session_durations
			WHERE actual_watch_ms > 0
			GROUP BY user_id
			HAVING total_actual_ms > 60000  -- At least 1 minute total
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate actual durations: " + err.Error()})
		}
		defer rows.Close()

		updated := 0
		totalWatchHours := float64(0)

		for rows.Next() {
			var userID string
			var totalActualMs, itemsWatched, totalSessions, avgSessionMs int64

			if err := rows.Scan(&userID, &totalActualMs, &itemsWatched, &totalSessions, &avgSessionMs); err != nil {
				log.Printf("Error scanning user data: %v", err)
				continue
			}

			// Insert the calculated actual watch time
			_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms) VALUES (?, ?)`,
				userID, totalActualMs)

			if err == nil {
				updated++
				totalWatchHours += float64(totalActualMs) / (1000.0 * 60.0 * 60.0)

				// Log detailed info for verification
				log.Printf("User %s: %.1f hours actual watch time (%d items, %d sessions, %.1f min avg)",
					userID,
					float64(totalActualMs)/(1000.0*60.0*60.0),
					itemsWatched,
					totalSessions,
					float64(avgSessionMs)/(1000.0*60.0))
			} else {
				log.Printf("Error inserting lifetime data for user %s: %v", userID, err)
			}
		}

		log.Printf("✓ Calculated actual watch durations for %d users (%.1f total hours)",
			updated, totalWatchHours)

		return c.JSON(fiber.Map{
			"success":           true,
			"users_updated":     updated,
			"total_watch_hours": totalWatchHours,
			"calculation_type":  "actual_duration",
			"timestamp":         time.Now().Unix(),
		})
	}
}
