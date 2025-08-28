package admin

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

// CleanupUsers removes all invalid/empty records from users and play events
func CleanupUsers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Count invalid records before cleanup - updated table names
		var invalidUsers, invalidPlaySessions, invalidPlayEvents, invalidPlayIntervals int
		db.QueryRow(`SELECT COUNT(*) FROM emby_user WHERE id = '' OR id IS NULL`).Scan(&invalidUsers)
		db.QueryRow(`SELECT COUNT(*) FROM play_sessions WHERE user_id = '' OR user_id IS NULL`).Scan(&invalidPlaySessions)
		db.QueryRow(`SELECT COUNT(*) FROM play_events WHERE session_fk IN (SELECT id FROM play_sessions WHERE user_id = '' OR user_id IS NULL)`).Scan(&invalidPlayEvents)
		db.QueryRow(`SELECT COUNT(*) FROM play_intervals WHERE user_id = '' OR user_id IS NULL`).Scan(&invalidPlayIntervals)

		// Clean up emby_user table
		result1, err1 := db.Exec(`DELETE FROM emby_user WHERE id = '' OR id IS NULL`)
		deletedUsers := int64(0)
		if err1 == nil {
			deletedUsers, _ = result1.RowsAffected()
		}

		// Clean up play_sessions table (this will cascade to play_events)
		result2, err2 := db.Exec(`DELETE FROM play_sessions WHERE user_id = '' OR user_id IS NULL`)
		deletedPlaySessions := int64(0)
		if err2 == nil {
			deletedPlaySessions, _ = result2.RowsAffected()
		}

		// Clean up play_intervals table
		result3, err3 := db.Exec(`DELETE FROM play_intervals WHERE user_id = '' OR user_id IS NULL`)
		deletedPlayIntervals := int64(0)
		if err3 == nil {
			deletedPlayIntervals, _ = result3.RowsAffected()
		}

		// Clean up lifetime_watch table
		result4, err4 := db.Exec(`DELETE FROM lifetime_watch WHERE user_id = '' OR user_id IS NULL`)
		deletedLifetimeWatch := int64(0)
		if err4 == nil {
			deletedLifetimeWatch, _ = result4.RowsAffected()
		}

		// Get final counts
		var finalUsers, finalPlaySessions, finalPlayEvents, finalPlayIntervals, finalLifetimeWatch int
		db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&finalUsers)
		db.QueryRow(`SELECT COUNT(*) FROM play_sessions`).Scan(&finalPlaySessions)
		db.QueryRow(`SELECT COUNT(*) FROM play_events`).Scan(&finalPlayEvents)
		db.QueryRow(`SELECT COUNT(*) FROM play_intervals`).Scan(&finalPlayIntervals)
		db.QueryRow(`SELECT COUNT(*) FROM lifetime_watch`).Scan(&finalLifetimeWatch)

		// Build error messages
		errors := []string{}
		if err1 != nil {
			errors = append(errors, "users: "+err1.Error())
		}
		if err2 != nil {
			errors = append(errors, "play_sessions: "+err2.Error())
		}
		if err3 != nil {
			errors = append(errors, "play_intervals: "+err3.Error())
		}
		if err4 != nil {
			errors = append(errors, "lifetime_watch: "+err4.Error())
		}

		return c.JSON(fiber.Map{
			"cleanup_results": fiber.Map{
				"invalid_users_found":          invalidUsers,
				"invalid_play_sessions_found":  invalidPlaySessions,
				"invalid_play_events_found":    invalidPlayEvents,
				"invalid_play_intervals_found": invalidPlayIntervals,
				"deleted_users":                deletedUsers,
				"deleted_play_sessions":        deletedPlaySessions,
				"deleted_play_intervals":       deletedPlayIntervals,
				"deleted_lifetime_watch":       deletedLifetimeWatch,
			},
			"final_counts": fiber.Map{
				"total_users":          finalUsers,
				"total_play_sessions":  finalPlaySessions,
				"total_play_events":    finalPlayEvents,
				"total_play_intervals": finalPlayIntervals,
				"total_lifetime_watch": finalLifetimeWatch,
			},
			"errors": errors,
		})
	}
}
