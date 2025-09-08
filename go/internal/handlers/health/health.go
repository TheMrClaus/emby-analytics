package health

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"time"

	"github.com/gofiber/fiber/v3"
)

type HealthStatus struct {
	OK            bool                `json:"ok"`
	Timestamp     string              `json:"timestamp"`
	Database      DatabaseHealth      `json:"database"`
	DataIntegrity DataIntegrityHealth `json:"data_integrity"`
	Performance   PerformanceHealth   `json:"performance"`
}

type DatabaseHealth struct {
	OK             bool   `json:"ok"`
	Error          string `json:"error,omitempty"`
	OpenConns      int    `json:"open_connections"`
	IdleConns      int    `json:"idle_connections"`
	ConnectionTime string `json:"connection_time"`
}

type DataIntegrityHealth struct {
	OK             bool   `json:"ok"`
	Error          string `json:"error,omitempty"`
	UserCount      int    `json:"user_count"`
	ItemCount      int    `json:"item_count"`
	SessionCount   int    `json:"session_count"`
	LastSessionAge string `json:"last_session_age"`
}

type PerformanceHealth struct {
	OK          bool   `json:"ok"`
	QueryTime   string `json:"query_time"`
	SlowQueries int    `json:"slow_queries"`
	Warning     string `json:"warning,omitempty"`
}

func Health(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		status := HealthStatus{
			OK:        true,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Test database connectivity
		dbStart := time.Now()
		err := db.Ping()
		dbDuration := time.Since(dbStart)

		status.Database.ConnectionTime = dbDuration.String()
		if err != nil {
			status.OK = false
			status.Database.OK = false
			status.Database.Error = err.Error()
			logging.Debug("Database ping failed: %v", err)
		} else {
			status.Database.OK = true

			// Get connection pool stats
			stats := db.Stats()
			status.Database.OpenConns = stats.OpenConnections
			status.Database.IdleConns = stats.Idle
		}

		// Test data integrity
		if status.Database.OK {
			dataOK := true
			var dataError string

			// Count basic entities
			err = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&status.DataIntegrity.UserCount)
			if err != nil {
				dataOK = false
				dataError = "Failed to count users: " + err.Error()
			}

			if dataOK {
				err = db.QueryRow(`SELECT COUNT(*) FROM library_item`).Scan(&status.DataIntegrity.ItemCount)
				if err != nil {
					dataOK = false
					dataError = "Failed to count library items: " + err.Error()
				}
			}

            if dataOK {
                err = db.QueryRow(`SELECT COUNT(*) FROM play_sessions WHERE COALESCE(item_type,'') NOT IN ('TvChannel','LiveTv','Channel','TvProgram')`).Scan(&status.DataIntegrity.SessionCount)
            if err != nil {
                dataOK = false
                dataError = "Failed to count sessions: " + err.Error()
            }
            }

			// Check for recent activity
			if dataOK {
				var lastSession sql.NullString
                err = db.QueryRow(`SELECT MAX(started_at) FROM play_sessions WHERE started_at IS NOT NULL AND COALESCE(item_type,'') NOT IN ('TvChannel','LiveTv','Channel','TvProgram')`).Scan(&lastSession)
				if err != nil {
					dataOK = false
					dataError = "Failed to get last session: " + err.Error()
				} else if lastSession.Valid {
					if lastTime, err := time.Parse("2006-01-02 15:04:05", lastSession.String); err == nil {
						status.DataIntegrity.LastSessionAge = time.Since(lastTime).String()
					}
				}
			}

			status.DataIntegrity.OK = dataOK
			status.DataIntegrity.Error = dataError
			if !dataOK {
				status.OK = false
			}
		}

		// Performance checks
		queryDuration := time.Since(start)
		status.Performance.QueryTime = queryDuration.String()
		status.Performance.OK = queryDuration < 5*time.Second

		if queryDuration > 2*time.Second {
			status.Performance.Warning = "Health check taking longer than expected"
			status.Performance.SlowQueries = 1
		}

		if !status.Performance.OK {
			status.OK = false
		}

		logging.Debug("Health check completed in %v, status=%t", queryDuration, status.OK)

		// Return appropriate HTTP status
		if !status.OK {
			return c.Status(503).JSON(status)
		}
		return c.JSON(status)
	}
}
