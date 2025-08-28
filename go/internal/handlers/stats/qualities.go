package stats

import (
	"database/sql"
	"regexp"

	"github.com/gofiber/fiber/v3"
)

// getQualityLabel now classifies by WIDTH to match the Emby plugin logic.
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
			return "Resolution Not Available"
		}
	}

	// Fallback: try to infer from DisplayTitle (e.g., "1080p H264").
	if displayTitle.Valid && displayTitle.String != "" {
		re := regexp.MustCompile(`(?i)\b(8k|4k|2160p|1440p|1080p|720p|576p|540p|480p|360p)\b`)
		if m := re.FindStringSubmatch(displayTitle.String); len(m) > 0 {
			switch m[1] {
			case "8k":
				return "8K"
			case "4k", "2160p":
				return "4K"
			case "1440p":
				// Not a width bucket in plugin; treat as 1080p for closest parity.
				return "1080p"
			case "1080p":
				return "1080p"
			case "720p":
				return "720p"
			case "576p", "540p", "480p", "360p":
				return "SD"
			}
		}
	}

	return "Unknown"
}

// Qualities returns counts grouped by quality label using WIDTH from library_item.
func Qualities(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		type row struct {
			Width        sql.NullInt64
			DisplayTitle sql.NullString
		}

		rows, err := db.Query(`
			SELECT
				width,
				display_title
			FROM library_item
			WHERE media_type = 'Video'
		`)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "query failed",
				"details": err.Error(),
			})
		}
		defer rows.Close()

		buckets := map[string]int{
			"8K":      0,
			"4K":      0,
			"1080p":   0,
			"720p":    0,
			"SD":      0,
			"Unknown": 0,
		}

		for rows.Next() {
			var r row
			if err := rows.Scan(&r.Width, &r.DisplayTitle); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   "scan failed",
					"details": err.Error(),
				})
			}
			label := getQualityLabel(r.Width, r.DisplayTitle)
			buckets[label]++
		}
		if err := rows.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "row iteration failed",
				"details": err.Error(),
			})
		}

		type QualityCount struct {
			Label string `json:"label"`
			Count int    `json:"count"`
		}
		out := []QualityCount{
			{Label: "8K", Count: buckets["8K"]},
			{Label: "4K", Count: buckets["4K"]},
			{Label: "1080p", Count: buckets["1080p"]},
			{Label: "720p", Count: buckets["720p"]},
			{Label: "SD", Count: buckets["SD"]},
			// If you chart Unknown, uncomment next line:
			// {Label: "Unknown", Count: buckets["Unknown"]},
		}

		return c.JSON(out)
	}
}
