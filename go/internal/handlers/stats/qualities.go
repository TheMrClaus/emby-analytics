package stats

import (
	"database/sql"
	"log"
	"regexp"

	"github.com/gofiber/fiber/v3"
)

/*
Fiber v3 notes:
- fiber.Handler is: func(fiber.Ctx) error
- main.go registers: app.Get("/api/stats/qualities", stats.Qualities(sqlDB))
  => Qualities MUST be a factory that returns fiber.Handler.
*/

// Qualities returns a Fiber handler that aggregates quality buckets.
// Keep main.go like: app.Get("/api/stats/qualities", stats.Qualities(sqlDB))
func Qualities(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		return qualitiesCore(c, db)
	}
}

// qualitiesCore executes a simple query to get Width and DisplayTitle for videos,
// buckets them using getQualityLabel, and returns JSON counts.
func qualitiesCore(c fiber.Ctx, db *sql.DB) error {
	// ---- EDIT HERE if your schema uses different names ----
	// This query expects a table with columns:
	//   Width (INTEGER/NULL) and DisplayTitle (TEXT/NULL).
	// Common guesses are Items, LibraryItems, or MediaItems.
	// Start with "Items" which many Emby-derived schemas use.
	const q = `
		SELECT
			Width,         -- INTEGER, may be NULL
			DisplayTitle   -- TEXT, may be NULL (e.g., "Avatar 4K 2160p")
		FROM Items
		WHERE 1=1
		  -- Uncomment if you have a MediaType column to limit to video rows:
		  -- AND MediaType = 'Video'
	`

	type row struct {
		Width        sql.NullInt64
		DisplayTitle sql.NullString
	}

	rows, err := db.Query(q)
	if err != nil {
		// If your table is named differently, return a helpful message.
		log.Printf("stats.qualities: query failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "query failed; verify table/column names (expected table 'Items' with columns Width, DisplayTitle)",
		})
	}
	defer rows.Close()

	// Buckets we report
	counts := map[string]int{
		"8K":                       0,
		"4K":                       0,
		"1080p":                    0,
		"720p":                     0,
		"SD":                       0,
		"Resolution Not Available": 0,
	}

	for rows.Next() {
		var r row
		if scanErr := rows.Scan(&r.Width, &r.DisplayTitle); scanErr != nil {
			log.Printf("stats.qualities: scan failed: %v", scanErr)
			continue
		}
		label := getQualityLabel(r.Width, r.DisplayTitle)
		if _, ok := counts[label]; !ok {
			// In case a future label appears, lump it into Unknown to avoid 500s.
			counts["Resolution Not Available"]++
			continue
		}
		counts[label]++
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		log.Printf("stats.qualities: rows error: %v", rowsErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "row iteration failed",
		})
	}

	return c.JSON(fiber.Map{
		"buckets": counts,
	})
}

// getQualityLabel classifies by WIDTH to match the Emby C# plugin logic.
// It also falls back to DisplayTitle parsing if width is absent.
func getQualityLabel(width sql.NullInt64, displayTitle sql.NullString) string {
	if width.Valid {
		w := int(width.Int64)
		switch {
		case w >= 3841 && w <= 7680:
			return "8K"
		case w >= 1921 && w <= 3840:
			return "4K"
		case w >= 1281 && w <= 1920:
			return "1080p"
		case w >= 1200 && w <= 1280:
			return "720p"
		case w > 0 && w < 1200:
			return "SD"
		default:
			// Unknown/unsupported width; fall through to fallback.
		}
	}

	// Fallback: infer from DisplayTitle (e.g., "8K", "4K", "2160p", "1080p", "720p", "SD/480p/576p").
	if displayTitle.Valid && displayTitle.String != "" {
		s := displayTitle.String

		// Check 8K first to avoid matching "4k" within "8k".
		re8k := regexp.MustCompile(`(?i)\b(8k|7680p|4320p)\b`)
		if re8k.MatchString(s) {
			return "8K"
		}

		// 4K can appear as "4k" or "2160p".
		re4k := regexp.MustCompile(`(?i)\b(4k|2160p)\b`)
		if re4k.MatchString(s) {
			return "4K"
		}

		re1080 := regexp.MustCompile(`(?i)\b(1080p)\b`)
		if re1080.MatchString(s) {
			return "1080p"
		}

		re720 := regexp.MustCompile(`(?i)\b(720p)\b`)
		if re720.MatchString(s) {
			return "720p"
		}

		// SD catch: common SD notations.
		reSD := regexp.MustCompile(`(?i)\b(sd|480p|576p)\b`)
		if reSD.MatchString(s) {
			return "SD"
		}
	}

	return "Resolution Not Available"
}
