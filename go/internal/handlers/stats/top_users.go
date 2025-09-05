package stats

import (
	"database/sql"
	"emby-analytics/internal/handlers/settings"
	"emby-analytics/internal/queries"
	"emby-analytics/internal/tasks"
	"sort"
	"time"

	"github.com/gofiber/fiber/v3"
)

type TopUser struct {
	UserID string  `json:"user_id"`
	Name   string  `json:"name"`
	Hours  float64 `json:"hours"`
}

func TopUsers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// --- Parameter Parsing ---
		timeframe := c.Query("timeframe", "")
		if timeframe == "" {
			// Fallback to days parameter if timeframe not provided
			days := parseQueryInt(c, "days", 14)
			if days <= 0 {
				timeframe = "all-time"
			} else if days == 1 {
				timeframe = "1d"
			} else if days == 3 {
				timeframe = "3d"
			} else if days == 7 {
				timeframe = "7d"
			} else if days == 14 {
				timeframe = "14d"
			} else if days == 30 {
				timeframe = "30d"
			} else {
				timeframe = "30d" // Default for large day values
			}
		}
		if timeframe == "" {
			timeframe = "14d" // Final fallback
		}

		limit := parseQueryInt(c, "limit", 10)
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		// --- "All-Time" Logic with dynamic Trakt calculation ---
		if timeframe == "all-time" {
			// Get the setting for whether to include Trakt items
			includeTrakt := settings.GetSettingBool(db, "include_trakt_items", false)

			rows, err := db.Query(`
				SELECT
					u.id,
					u.name,
					CASE WHEN ? = 1 THEN 
						COALESCE((lw.emby_ms + lw.trakt_ms) / 3600000.0, 0)
					ELSE 
						COALESCE(lw.emby_ms / 3600000.0, 0) 
					END AS hours
				FROM emby_user u
				LEFT JOIN lifetime_watch lw ON lw.user_id = u.id
				WHERE lw.emby_ms > 0 OR lw.trakt_ms > 0
				ORDER BY hours DESC
				LIMIT ?;
			`, includeTrakt, limit)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			defer rows.Close()

			out := []TopUser{}
			for rows.Next() {
				var u TopUser
				if err := rows.Scan(&u.UserID, &u.Name, &u.Hours); err != nil {
					return c.Status(500).JSON(fiber.Map{"error": err.Error()})
				}
				out = append(out, u)
			}
			return c.JSON(out)
		}

		// --- Live-Aware Time-Windowed Logic ---
		days := parseTimeframeToDays(timeframe)
		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -days).Unix()

		// 1. Get historical data from the database (fetch a high number to merge before limiting)
		historicalRows, err := queries.TopUsersByWatchSeconds(c, db, winStart, winEnd, 1000)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		if err != nil || len(historicalRows) == 0 {
			// Fallback to counting sessions if intervals aren't populated
			rows, err := db.Query(`
        SELECT 
            u.id, 
            u.name, 
            COUNT(DISTINCT ps.id) * 0.5 as hours
        FROM emby_user u
        LEFT JOIN play_sessions ps ON ps.user_id = u.id
        WHERE ps.started_at >= ? AND ps.started_at <= ?
        GROUP BY u.id, u.name
        ORDER BY hours DESC
        LIMIT ?
    `, winStart, winEnd, 1000)

			if err == nil {
				defer rows.Close()
				historicalRows = []queries.TopUserRow{}
				for rows.Next() {
					var r queries.TopUserRow
					if err := rows.Scan(&r.UserID, &r.Name, &r.Hours); err == nil {
						historicalRows = append(historicalRows, r)
					}
				}
			}
		}

		// 2. Prepare to combine historical and live data
		combinedHours := make(map[string]float64)
		userNames := make(map[string]string)

		for _, row := range historicalRows {
			combinedHours[row.UserID] += row.Hours
			userNames[row.UserID] = row.Name
		}

		// 3. Get live data from the Intervalizer and merge it
		liveWatchTimes := tasks.GetLiveUserWatchTimes() // Returns seconds
		for userID, seconds := range liveWatchTimes {
			combinedHours[userID] += seconds / 3600.0 // Convert seconds to hours
			// Ensure we have a username, even if the user only has a live session
			if _, ok := userNames[userID]; !ok {
				var name string
				// This query is fast and only runs for new users with live sessions
				_ = db.QueryRow("SELECT name FROM emby_user WHERE id = ?", userID).Scan(&name)
				userNames[userID] = name
			}
		}

		// 4. Convert the combined map back to a slice for sorting
		finalResult := make([]TopUser, 0, len(combinedHours))
		for userID, hours := range combinedHours {
			if userNames[userID] != "" { // Only include users we have a name for
				finalResult = append(finalResult, TopUser{
					UserID: userID,
					Name:   userNames[userID],
					Hours:  hours,
				})
			}
		}

		// 5. Sort the final combined list by hours, descending
		sort.Slice(finalResult, func(i, j int) bool {
			return finalResult[i].Hours > finalResult[j].Hours
		})

		// 6. Apply the final limit
		if len(finalResult) > limit {
			finalResult = finalResult[:limit]
		}

		return c.JSON(finalResult)
	}
}
