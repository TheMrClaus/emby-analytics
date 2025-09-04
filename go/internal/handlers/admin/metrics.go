package admin

import (
	"database/sql"
	"log"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v3"
)

type SystemMetrics struct {
	Timestamp    string           `json:"timestamp"`
	Database     DatabaseMetrics  `json:"database"`
	Runtime      RuntimeMetrics   `json:"runtime"`
	Performance  PerformanceMetrics `json:"performance"`
}

type DatabaseMetrics struct {
	OpenConnections     int    `json:"open_connections"`
	InUse               int    `json:"in_use"`
	Idle                int    `json:"idle"`
	WaitCount           int64  `json:"wait_count"`
	WaitDuration        string `json:"wait_duration"`
	MaxIdleClosed       int64  `json:"max_idle_closed"`
	MaxIdleTimeClosed   int64  `json:"max_idle_time_closed"`
	MaxLifetimeClosed   int64  `json:"max_lifetime_closed"`
	MaxOpenConnections  int    `json:"max_open_connections"`
	QueryCount          int    `json:"query_count,omitempty"` // Custom counter
	SlowQueryCount      int    `json:"slow_query_count,omitempty"` // Custom counter
}

type RuntimeMetrics struct {
	GoVersion    string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutine"`
	NumCPU       int    `json:"num_cpu"`
	MemStats     struct {
		Alloc        uint64 `json:"alloc_bytes"`
		TotalAlloc   uint64 `json:"total_alloc_bytes"`
		Sys          uint64 `json:"sys_bytes"`
		NumGC        uint32 `json:"num_gc"`
	} `json:"mem_stats"`
}

type PerformanceMetrics struct {
	UptimeSeconds     float64            `json:"uptime_seconds"`
	AvgResponseTime   string             `json:"avg_response_time"`
	ErrorCount        int                `json:"error_count,omitempty"`
	RequestCounts     map[string]int     `json:"request_counts,omitempty"`
	SlowEndpoints     []SlowEndpointInfo `json:"slow_endpoints,omitempty"`
}

type SlowEndpointInfo struct {
	Path         string `json:"path"`
	Count        int    `json:"count"`
	AvgDuration  string `json:"avg_duration"`
	LastOccurred string `json:"last_occurred"`
}

var (
	appStartTime = time.Now()
	queryMetrics = struct {
		totalQueries   int
		slowQueries    int
		totalDuration  time.Duration
	}{}
)

// IncrementQueryMetrics should be called from query handlers to track performance
func IncrementQueryMetrics(duration time.Duration, isSlowQuery bool) {
	queryMetrics.totalQueries++
	queryMetrics.totalDuration += duration
	if isSlowQuery {
		queryMetrics.slowQueries++
	}
}

func SystemMetricsHandler(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		metrics := SystemMetrics{
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Database metrics from connection pool
		if db != nil {
			stats := db.Stats()
			metrics.Database = DatabaseMetrics{
				OpenConnections:     stats.OpenConnections,
				InUse:               stats.InUse,
				Idle:                stats.Idle,
				WaitCount:           stats.WaitCount,
				WaitDuration:        stats.WaitDuration.String(),
				MaxIdleClosed:       stats.MaxIdleClosed,
				MaxIdleTimeClosed:   stats.MaxIdleTimeClosed,
				MaxLifetimeClosed:   stats.MaxLifetimeClosed,
				MaxOpenConnections:  stats.MaxOpenConnections,
				QueryCount:          queryMetrics.totalQueries,
				SlowQueryCount:      queryMetrics.slowQueries,
			}
		}

		// Runtime metrics
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		metrics.Runtime = RuntimeMetrics{
			GoVersion:    runtime.Version(),
			NumGoroutine: runtime.NumGoroutine(),
			NumCPU:       runtime.NumCPU(),
		}
		
		metrics.Runtime.MemStats.Alloc = m.Alloc
		metrics.Runtime.MemStats.TotalAlloc = m.TotalAlloc
		metrics.Runtime.MemStats.Sys = m.Sys
		metrics.Runtime.MemStats.NumGC = m.NumGC

		// Performance metrics
		uptime := time.Since(appStartTime)
		metrics.Performance = PerformanceMetrics{
			UptimeSeconds: uptime.Seconds(),
		}
		
		if queryMetrics.totalQueries > 0 {
			avgDuration := queryMetrics.totalDuration / time.Duration(queryMetrics.totalQueries)
			metrics.Performance.AvgResponseTime = avgDuration.String()
		}

		// Log metrics periodically
		log.Printf("[metrics] DB connections: open=%d, in_use=%d, idle=%d, wait_count=%d",
			metrics.Database.OpenConnections,
			metrics.Database.InUse, 
			metrics.Database.Idle,
			metrics.Database.WaitCount)

		return c.JSON(metrics)
	}
}