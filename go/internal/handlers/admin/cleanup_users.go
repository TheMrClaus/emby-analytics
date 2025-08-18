package admin

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

// CleanupUsers removes all invalid/empty records from users and play_events
func CleanupUsers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Count invalid records before cleanup
		var invalidUsers, invalidPlayEvents int
		db.QueryRow(`SELECT COUNT(*) FROM emby_user WHERE id = '' OR id IS NULL`).Scan(&invalidUsers)
		db.QueryRow(`SELECT COUNT(*) FROM play_event WHERE user_id = '' OR user_id IS NULL`).Scan(&invalidPlayEvents)

		// Clean up emby_user table
		result1, err1 := db.Exec(`DELETE FROM emby_user WHERE id = '' OR id IS NULL`)
		deletedUsers := int64(0)
		if err1 == nil {
			deletedUsers, _ = result1.RowsAffected()
		}

		// Clean up play_event table
		result2, err2 := db.Exec(`DELETE FROM play_event WHERE user_id = '' OR user_id IS NULL`)
		deletedPlayEvents := int64(0)
		if err2 == nil {
			deletedPlayEvents, _ = result2.RowsAffected()
		}

		// Clean up lifetime_watch table
		result3, err3 := db.Exec(`DELETE FROM lifetime_watch WHERE user_id = '' OR user_id IS NULL`)
		deletedLifetimeWatch := int64(0)
		if err3 == nil {
			deletedLifetimeWatch, _ = result3.RowsAffected()
		}

		// Get final counts
		var finalUsers, finalPlayEvents, finalLifetimeWatch int
		db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&finalUsers)
		db.QueryRow(`SELECT COUNT(*) FROM play_event`).Scan(&finalPlayEvents)
		db.QueryRow(`SELECT COUNT(*) FROM lifetime_watch`).Scan(&finalLifetimeWatch)

		// Build error messages
		errors := []string{}
		if err1 != nil {
			errors = append(errors, "users: "+err1.Error())
		}
		if err2 != nil {
			errors = append(errors, "play_events: "+err2.Error())
		}
		if err3 != nil {
			errors = append(errors, "lifetime_watch: "+err3.Error())
		}

		return c.JSON(fiber.Map{
			"cleanup_results": fiber.Map{
				"invalid_users_found":       invalidUsers,
				"invalid_play_events_found": invalidPlayEvents,
				"deleted_users":             deletedUsers,
				"deleted_play_events":       deletedPlayEvents,
				"deleted_lifetime_watch":    deletedLifetimeWatch,
			},
			"final_counts": fiber.Map{
				"total_users":          finalUsers,
				"total_play_events":    finalPlayEvents,
				"total_lifetime_watch": finalLifetimeWatch,
			},
			"errors": errors,
		})
	}
}
