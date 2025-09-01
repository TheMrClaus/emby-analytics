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

		// Check if enhanced columns exist by checking table structure
		var hasVideoMethod bool
		row := db.QueryRow(`
			SELECT COUNT(*) 
			FROM pragma_table_info('play_sessions') 
			WHERE name = 'video_method'
		`)
		var count int
		if err := row.Scan(&count); err == nil && count > 0 {
			hasVideoMethod = true
		}

		if !hasVideoMethod {
			log.Printf("[PlayMethods] Enhanced columns not found, using legacy mode")
			return legacyPlayMethods(c, db, days)
		}

		// Enhanced query with new columns - fixed to use Unix timestamp properly
		query := `
			SELECT
				COALESCE(video_method, 'DirectPlay') AS video_method,
				COALESCE(audio_method, 'DirectPlay') AS audio_method,
				COUNT(*) AS cnt
			FROM play_sessions
			WHERE started_at >= (strftime('%s','now') - (? * 86400))
				AND started_at IS NOT NULL
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
			"DirectStream":  0,
			"Transcode":     0,
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
				continue
			}

			// Create detailed key
			key := fmt.Sprintf("%s|%s", videoMethod, audioMethod)
			methodBreakdown[key] = cnt

			// Categorize for high-level summary
			if videoMethod == "DirectPlay" && audioMethod == "DirectPlay" {
				summary["DirectPlay"] += cnt
			} else if videoMethod == "DirectStream" && audioMethod == "DirectPlay" {
				summary["DirectStream"] += cnt // Fixed: was VideoOnly
			} else if videoMethod == "Transcode" && audioMethod == "DirectPlay" {
				summary["VideoOnly"] += cnt
			} else if videoMethod == "DirectPlay" && audioMethod == "Transcode" {
				summary["AudioOnly"] += cnt
			} else if videoMethod == "DirectStream" && audioMethod == "Transcode" {
				summary["Transcode"] += cnt // Remux with audio transcode
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

		// Ensure we have the basic methods even if not in data
		if summary["DirectPlay"] == 0 && summary["DirectStream"] == 0 && summary["Transcode"] == 0 {
			// If no data, try legacy mode as fallback
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
			AND started_at IS NOT NULL
		GROUP BY raw_method
	`

	rows, err := db.Query(query, days)
	if err != nil {
		log.Printf("[PlayMethods] Legacy query failed: %v", err)
		// Return empty data instead of error
		return c.JSON(fiber.Map{
			"methods": map[string]int{
				"DirectPlay":   0,
				"DirectStream": 0,
				"Transcode":    0,
				"Unknown":      0,
			},
			"detailed": make(map[string]int),
			"days":     days,
		})
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
			log.Printf("[PlayMethods] Legacy scan error: %v", err)
			continue
		}
		normalized := normalize(raw)
		out[normalized] += cnt
	}

	if err := rows.Err(); err != nil {
		log.Printf("[PlayMethods] Legacy rows error: %v", err)
	}

	// Return in the format expected by frontend
	return c.JSON(fiber.Map{
		"methods":  out, // Return the simple format for legacy
		"detailed": make(map[string]int),
		"days":     days,
	})
}
