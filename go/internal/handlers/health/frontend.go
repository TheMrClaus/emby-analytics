package health

import (
	"database/sql"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
)

type FrontendHealthStatus struct {
	OK           bool   `json:"ok"`
	Timestamp    string `json:"timestamp"`
	Overview     bool   `json:"overview_data_available"`
	UsersData    bool   `json:"users_data_available"`
	SessionsData bool   `json:"sessions_data_available"`
	ResponseTime string `json:"response_time"`
	Error        string `json:"error,omitempty"`
}

// FrontendHealth performs a lightweight check that simulates what the frontend needs
func FrontendHealth(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		status := FrontendHealthStatus{
			OK:        true,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Test overview data (what OverviewCards component needs)
		var userCount, itemCount, sessionCount int
		err := db.QueryRow(`SELECT 
			(SELECT COUNT(*) FROM emby_user),
			(SELECT COUNT(*) FROM library_item WHERE media_type NOT IN ('TvChannel', 'LiveTv', 'Channel')),
			(SELECT COUNT(*) FROM play_sessions WHERE started_at IS NOT NULL)
		`).Scan(&userCount, &itemCount, &sessionCount)
		
		if err != nil {
			status.OK = false
			status.Error = "Failed to fetch overview data: " + err.Error()
			log.Printf("[health-frontend] Overview data check failed: %v", err)
		} else {
			status.Overview = true
			if userCount == 0 && itemCount == 0 && sessionCount == 0 {
				status.Error = "All counts are zero - data may not be syncing properly"
			}
		}

		// Quick check for recent data activity
		if status.Overview {
			var recentSessions int
			err = db.QueryRow(`SELECT COUNT(*) FROM play_sessions WHERE started_at > datetime('now', '-24 hours')`).Scan(&recentSessions)
			if err == nil {
				status.SessionsData = recentSessions > 0
			}

			var activeUsers int 
			err = db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM play_sessions WHERE started_at > datetime('now', '-7 days')`).Scan(&activeUsers)
			if err == nil {
				status.UsersData = activeUsers > 0
			}
		}

		status.ResponseTime = time.Since(start).String()
		
		// Log results for monitoring
		log.Printf("[health-frontend] Check completed in %v: users=%d, items=%d, sessions=%d, status=%t",
			time.Since(start), userCount, itemCount, sessionCount, status.OK)

		if !status.OK {
			return c.Status(503).JSON(status)
		}
		return c.JSON(status)
	}
}