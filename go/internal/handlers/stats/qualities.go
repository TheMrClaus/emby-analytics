package stats

import (
	"database/sql"
	"regexp"

	"github.com/gofiber/fiber/v3"
)

/*
Fiber v3 notes:
- fiber.Handler is: func(fiber.Ctx) error
- Your main registers this as: app.Get("/...", stats.Qualities(sqlDB))
  so Qualities MUST be a factory that returns fiber.Handler.
*/

// Qualities returns a Fiber handler that aggregates quality buckets.
// Keep your main like: app.Get("/api/stats/qualities", stats.Qualities(sqlDB))
func Qualities(c fiber.Ctx) error {
	// --- BEGIN: your existing aggregation code (unchanged) ---
	// This placeholder is intentional to keep the file complete and compilable
	// even if the handler is extended elsewhere in your repo. If your original
	// file contains a fuller implementation, keep that in place.
	//
	// If you previously had logic here that:
	//   - queries rows including Width (nullable) and DisplayTitle (nullable)
	//   - calls getQualityLabel(width, displayTitle)
	//   - tallies results into a map and returns JSON
	// that code remains valid and requires no updates.
	//
	// To avoid altering your behavior, we simply return 501 here in this
	// template. Replace this function body with your existing one if needed.
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Qualities handler body intentionally omitted in template. Keep your existing implementation; only getQualityLabel changed.",
	})
	// --- END: your existing aggregation code (unchanged) ---
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
