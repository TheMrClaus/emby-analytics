package stats

import (
	"database/sql"
	"emby-analytics/internal/handlers/settings"

	"github.com/gofiber/fiber/v3"
)

type UserWatchTime struct {
	UserID     string  `json:"user_id"`
	Name       string  `json:"name"`
	Hours      float64 `json:"hours"`
	EmbyHours  float64 `json:"emby_hours"`
	TraktHours float64 `json:"trakt_hours"`
}

// UserWatchTimeHandler returns watch time for a specific user with dynamic Trakt inclusion
func UserWatchTimeHandler(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := c.Params("id")
		if userID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "User ID is required"})
		}

		// Get the setting for whether to include Trakt items
		includeTrakt := settings.GetSettingBool(db, "include_trakt_items", false)

		var user UserWatchTime
		err := db.QueryRow(`
			SELECT
				u.id,
				u.name,
				COALESCE(lw.emby_ms, 0) / 3600000.0 AS emby_hours,
				COALESCE(lw.trakt_ms, 0) / 3600000.0 AS trakt_hours
			FROM emby_user u
			LEFT JOIN lifetime_watch lw ON lw.user_id = u.id
			WHERE u.id = ?
		`, userID).Scan(&user.UserID, &user.Name, &user.EmbyHours, &user.TraktHours)

		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "User not found"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Calculate total hours based on setting
		if includeTrakt {
			user.Hours = user.EmbyHours + user.TraktHours
		} else {
			user.Hours = user.EmbyHours
		}

		return c.JSON(user)
	}
}

// AllUsersWatchTimeHandler returns watch time for all users with dynamic Trakt inclusion
func AllUsersWatchTimeHandler(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get the setting for whether to include Trakt items
		includeTrakt := settings.GetSettingBool(db, "include_trakt_items", false)

		limit := parseQueryInt(c, "limit", 100)
		if limit <= 0 || limit > 1000 {
			limit = 100
		}

		rows, err := db.Query(`
			SELECT
				u.id,
				u.name,
				COALESCE(lw.emby_ms, 0) / 3600000.0 AS emby_hours,
				COALESCE(lw.trakt_ms, 0) / 3600000.0 AS trakt_hours
			FROM emby_user u
			LEFT JOIN lifetime_watch lw ON lw.user_id = u.id
			WHERE lw.emby_ms > 0 OR lw.trakt_ms > 0
			ORDER BY 
				CASE WHEN ? = 1 THEN (COALESCE(lw.emby_ms, 0) + COALESCE(lw.trakt_ms, 0))
				     ELSE COALESCE(lw.emby_ms, 0) END DESC
			LIMIT ?
		`, includeTrakt, limit)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		users := []UserWatchTime{}
		for rows.Next() {
			var user UserWatchTime
			if err := rows.Scan(&user.UserID, &user.Name, &user.EmbyHours, &user.TraktHours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Calculate total hours based on setting
			if includeTrakt {
				user.Hours = user.EmbyHours + user.TraktHours
			} else {
				user.Hours = user.EmbyHours
			}

			users = append(users, user)
		}

		return c.JSON(users)
	}
}
