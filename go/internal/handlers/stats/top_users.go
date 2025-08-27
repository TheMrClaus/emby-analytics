package stats

import (
	"database/sql"
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
		timeframe := c.Query("timeframe", "14d")
		limit := parseQueryInt(c, "limit", 10)
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		if timeframe == "all-time" {
			rows, err := db.Query(`
				SELECT
					u.id,
					u.name,
					COALESCE(lw.total_ms / 3600000.0, 0) AS hours
				FROM emby_user u
				LEFT JOIN lifetime_watch lw ON lw.user_id = u.id
				WHERE lw.total_ms > 0
				ORDER BY hours DESC
				LIMIT ?;
			`, limit)
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

		days := parseTimeframeToDays(timeframe)
		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -days).Unix()

		// 1. Get historical data from the database
		// CORRECTED: Pass 'c' directly.
		historicalRows, err := queries.TopUsersByWatchSeconds(c, db, winStart, winEnd, 1000)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		combinedHours := make(map[string]float64)
		userNames := make(map[string]string)
		for _, row := range historicalRows {
			combinedHours[row.UserID] += row.Hours
			userNames[row.UserID] = row.Name
		}

		liveWatchTimes := tasks.GetLiveUserWatchTimes()
		for userID, seconds := range liveWatchTimes {
			combinedHours[userID] += seconds / 3600.0
			if _, ok := userNames[userID]; !ok {
				var name string
				_ = db.QueryRow("SELECT name FROM emby_user WHERE id = ?", userID).Scan(&name)
				userNames[userID] = name
			}
		}

		finalResult := make([]TopUser, 0, len(combinedHours))
		for userID, hours := range combinedHours {
			finalResult = append(finalResult, TopUser{
				UserID: userID,
				Name:   userNames[userID],
				Hours:  hours,
			})
		}

		sort.Slice(finalResult, func(i, j int) bool {
			return finalResult[i].Hours > finalResult[j].Hours
		})

		if len(finalResult) > limit {
			finalResult = finalResult[:limit]
		}

		return c.JSON(finalResult)
	}
}
