package stats

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// normalize maps various casings/variants into our 4 buckets.
func normalize(method string) string {
	m := strings.ToLower(strings.TrimSpace(method))
	switch m {
	case "directplay", "direct play":
		return "DirectPlay"
	case "directstream", "direct stream":
		return "DirectStream"
	case "transcode", "transcoding":
		return "Transcode"
	default:
		return "Unknown"
	}
}

// PlayMethods returns a breakdown of playback methods over the last N days (default 30).
// Response: { "methods": { "DirectPlay": N, "DirectStream": N, "Transcode": N, "Unknown": N } }
func PlayMethods(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Fiber v3: parse query param manually
		daysStr := c.Query("days", "30")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			days = 30
		}

		// Prefer using play_sessions.play_method + started_at (unix seconds)
		// started_at/ended_at are defined in the schema migrations.
		// We count sessions whose started_at is within the last N days.
		query := `
			SELECT
				COALESCE(play_method, '') AS raw_method,
				COUNT(*) AS cnt
			FROM play_sessions
			WHERE started_at >= (strftime('%s','now') - (? * 86400))
			GROUP BY raw_method
		`

		rows, err := db.Query(query, days)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := map[string]int{
			"DirectPlay":   0,
			"DirectStream": 0,
			"Transcode":    0,
			"Unknown":      0,
		}

		for rows.Next() {
			var raw string
			var cnt int
			if err := rows.Scan(&raw, &cnt); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			out[normalize(raw)] += cnt
		}
		if err := rows.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"methods": out})
	}
}
