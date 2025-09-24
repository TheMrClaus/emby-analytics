package admin

import (
	"database/sql"
	"strconv"

	"emby-analytics/internal/audit"
	"github.com/gofiber/fiber/v3"
)

// GetCleanupJobs returns a list of recent cleanup jobs
// GET /admin/cleanup/jobs?limit=50
func GetCleanupJobs(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := 50
		if v := c.Query("limit", ""); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}

		jobs, err := audit.GetCleanupJobs(db, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"jobs":  jobs,
			"count": len(jobs),
			"limit": limit,
		})
	}
}

// GetCleanupJobDetails returns detailed information about a specific cleanup job
// GET /admin/cleanup/jobs/{jobId}
func GetCleanupJobDetails(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		jobID := c.Params("jobId")
		if jobID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "job ID required"})
		}

		job, items, err := audit.GetCleanupJobDetails(db, jobID)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "job not found"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"job":   job,
			"items": items,
			"count": len(items),
		})
	}
}
