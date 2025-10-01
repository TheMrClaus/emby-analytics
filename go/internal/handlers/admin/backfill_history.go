package admin

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"strconv"
	"strings"
	"time"

	"emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
)

// BackfillHistory fetches historical data from Emby for a specified number of days
func BackfillHistory(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		days := parseQueryInt(c, "days", 90) // Default to 90 days
		if days <= 0 || days > 365 {
			days = 90
		}

		logging.Debug("Starting backfill for %d days", days)
		startTime := time.Now()

		// Get all users
		users, err := em.GetUsers()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to get users: " + err.Error()})
		}

		totalEvents := 0
		apiCalls := 1 // GetUsers call
		processedUsers := 0

		for _, user := range users {
			if strings.TrimSpace(user.Id) == "" {
				continue
			}

			logging.Debug("Processing user: %s (ID: %s)", user.Name, user.Id)

			// Get historical data for this user
			history, err := em.GetUserPlayHistory(user.Id, days)
			apiCalls++

			if err != nil {
				logging.Debug("Error getting history for %s: %v", user.Name, err)
				continue
			}

			logging.Debug("User %s returned %d history items", user.Name, len(history))

			// Debug: show first few items
			for i, h := range history {
				if i < 3 { // Only show first 3 items for debugging
					logging.Debug("[backfill] Item %d: %s (%s) - DatePlayed: %s, PlaybackPos: %d",
						i+1, h.Name, h.Type, h.DatePlayed, h.PlaybackPos)
				}
			}

			processedUsers++
			userEvents := 0

			for _, h := range history {
				// Upsert user and item info
				upsertUserAndItem(db, user.Id, user.Name, h.Id, h.Name, h.Type)

				// Convert PlaybackPositionTicks to ms
				posMs := int64(0)
				if h.PlaybackPos > 0 {
					posMs = h.PlaybackPos / 10_000
				}

				// Parse DatePlayed to get timestamp
				var ts int64 = time.Now().UnixMilli() // fallback
				if h.DatePlayed != "" {
					if parsedTime, err := time.Parse(time.RFC3339, h.DatePlayed); err == nil {
						ts = parsedTime.UnixMilli()
					} else {
						logging.Debug("Failed to parse DatePlayed '%s': %v", h.DatePlayed, err)
					}
				}

				// Insert play event with actual DatePlayed timestamp
				if insertPlayEventWithTime(db, ts, user.Id, h.Id, posMs) {
					userEvents++
					totalEvents++
					logging.Debug("Inserted event: %s watched %s (pos: %dms)", user.Name, h.Name, posMs)
				} else {
					logging.Debug("Failed to insert event for %s - %s", user.Name, h.Name)
				}
			}

			logging.Debug("User %s: %d events inserted", user.Name, userEvents)
		}

		duration := time.Since(startTime)

		return c.JSON(fiber.Map{
			"success":         true,
			"days_requested":  days,
			"users_processed": processedUsers,
			"total_events":    totalEvents,
			"api_calls":       apiCalls,
			"duration_ms":     duration.Milliseconds(),
		})
	}
}

// Helper function to insert play event with custom timestamp
func insertPlayEventWithTime(db *sql.DB, ts int64, userID, itemID string, posMs int64) bool {
	res, err := db.Exec(`INSERT OR IGNORE INTO play_event (ts, user_id, item_id, pos_ms)
	                     VALUES (?, ?, ?, ?)`, ts, userID, itemID, posMs)
	if err != nil {
		return false
	}
	rows, _ := res.RowsAffected()
	return rows > 0
}

// Helper function (reuse from tasks/sync.go)
func upsertUserAndItem(db *sql.DB, userID, userName, itemID, itemName, itemType string) {
	if strings.TrimSpace(userID) != "" && strings.TrimSpace(userName) != "" {
		db.Exec(`INSERT OR IGNORE INTO emby_user (id, name) VALUES (?, ?)`, userID, userName)
	}
	if strings.TrimSpace(itemID) != "" && strings.TrimSpace(itemName) != "" {
		db.Exec(`INSERT OR IGNORE INTO library_item (id, name, type) VALUES (?, ?, ?)`, itemID, itemName, itemType)
	}
}

// parseQueryInt helper function for Fiber v3 compatibility
func parseQueryInt(c fiber.Ctx, key string, defaultValue int) int {
	str := c.Query(key)
	if str == "" {
		return defaultValue
	}

	if val, err := strconv.Atoi(str); err == nil {
		return val
	}
	return defaultValue
}
