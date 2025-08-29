package stats

import (
	"database/sql"
	"fmt"
	"log"
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
func PlayMethods(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Fiber v3: parse query param manually
		daysStr := c.Query("days", "30")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			days = 30
		}

		// Check if enhanced columns exist
		var testCol string
		err = db.QueryRow("SELECT video_method FROM play_sessions LIMIT 1").Scan(&testCol)
		hasEnhancedColumns := (err == nil)

		if !hasEnhancedColumns {
			log.Printf("[PlayMethods] Enhanced columns not found, using legacy mode")
			return legacyPlayMethods(c, db, days)
		}

		// Enhanced query with new columns
		query := `
			SELECT
				COALESCE(video_method, 'DirectPlay') AS video_method,
				COALESCE(audio_method, 'DirectPlay') AS audio_method,
				COUNT(*) AS cnt
			FROM play_sessions
			WHERE started_at >= (strftime('%s','now') - (? * 86400))
			GROUP BY video_method, audio_method
		`

		rows, err := db.Query(query, days)
		if err != nil {
			log.Printf("[PlayMethods] Enhanced query failed: %v", err)
			return legacyPlayMethods(c, db, days)
		}
		defer rows.Close()

		// Detailed breakdown
		methodBreakdown := make(map[string]int)

		// High-level summary
		summary := map[string]int{
			"DirectPlay":    0,
			"VideoOnly":     0,
			"AudioOnly":     0,
			"BothTranscode": 0,
			"Unknown":       0,
		}

		// Process results with proper variable declarations
		for rows.Next() {
			var videoMethod, audioMethod string
			var cnt int

			if err := rows.Scan(&videoMethod, &audioMethod, &cnt); err != nil {
				log.Printf("[PlayMethods] Scan error: %v", err)
				return legacyPlayMethods(c, db, days)
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
			log.Printf("[PlayMethods] Rows error: %v", err)
			return legacyPlayMethods(c, db, days)
		}

		return c.JSON(fiber.Map{
			"methods":  summary,
			"detailed": methodBreakdown,
			"days":     days,
		})
	}
}

// legacyPlayMethods provides the original functionality when new columns don't exist
func legacyPlayMethods(c fiber.Ctx, db *sql.DB, days int) error {
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

	// Convert to new format for compatibility
	summary := map[string]int{
		"DirectPlay":    out["DirectPlay"] + out["DirectStream"],
		"VideoOnly":     0,
		"AudioOnly":     0,
		"BothTranscode": out["Transcode"],
		"Unknown":       out["Unknown"],
	}

	return c.JSON(fiber.Map{
		"methods":  summary,
		"detailed": make(map[string]int), // Empty detailed for legacy
		"days":     days,
	})
}
