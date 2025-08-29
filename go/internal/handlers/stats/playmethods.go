package stats

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// normalize maps various casings/variants into our 4 buckets.
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
	case "direct": // handles the "Direct" value from Emby
		return "DirectPlay"
	case "remux", "copy", "directcopy":
		return "DirectStream"
	case "convert", "encoding":
		return "Transcode"
	case "":
		return "Unknown"
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

		// Enhanced query to get detailed method breakdown
		query := `
			SELECT
				COALESCE(video_method, 'DirectPlay') AS video_method,
				COALESCE(audio_method, 'DirectPlay') AS audio_method,
				COALESCE(play_method, 'Unknown') AS overall_method,
				COUNT(*) AS cnt
			FROM play_sessions
			WHERE started_at >= (strftime('%s','now') - (? * 86400))
			GROUP BY video_method, audio_method, overall_method
		`

		rows, err := db.Query(query, days)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		// Detailed breakdown
		methodBreakdown := make(map[string]int)

		// High-level summary for backwards compatibility
		summary := map[string]int{
			"DirectPlay":    0,
			"VideoOnly":     0,
			"AudioOnly":     0,
			"BothTranscode": 0,
			"Unknown":       0,
		}

		// Process results
		for rows.Next() {
			var videoMethod, audioMethod, overallMethod string
			var cnt int
			if err := rows.Scan(&videoMethod, audioMethod, overallMethod, &cnt); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Create detailed key
			key := fmt.Sprintf("%s|%s", videoMethod, audioMethod)
			methodBreakdown[key] = cnt

			// Categorize for high-level summary
			if videoMethod == "DirectPlay" && audioMethod == "DirectPlay" {
				summary["DirectPlay"] += cnt
			} else if videoMethod == "Transcode" && audioMethod == "DirectPlay" {
				summary["VideoOnly"] += cnt
			} else if videoMethod == "DirectPlay" && audioMethod == "Transcode" {
				summary["AudioOnly"] += cnt
			} else if videoMethod == "Transcode" && audioMethod == "Transcode" {
				summary["BothTranscode"] += cnt
			} else {
				summary["Unknown"] += cnt
			}
		}
		if err := rows.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"methods":  summary,         // High-level categories for charts
			"detailed": methodBreakdown, // Detailed breakdown
			"days":     days,
		})
	}
}
