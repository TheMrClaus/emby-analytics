package stats

import (
	"database/sql"
	"log"
	"net/url"

	"github.com/gofiber/fiber/v3"
)

// ItemsByQualityResponse holds the response structure for quality-based queries
type ItemsByQualityResponse struct {
	Items       []LibraryItemResponse `json:"items"`
	Total       int                   `json:"total"`
	Quality     string                `json:"quality"`
	HeightRange string                `json:"height_range"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"page_size"`
}

// ItemsByQuality returns all library items for a specific quality/resolution with pagination
func ItemsByQuality(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		quality := c.Params("quality")
		log.Printf("DEBUG: Received quality parameter: %q (length: %d)", quality, len(quality))

		// Decode URL parameter for Fiber v3
		decodedQuality, err := url.QueryUnescape(quality)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid quality parameter encoding"})
		}
		quality = decodedQuality
		log.Printf("DEBUG: Decoded quality parameter: %q", quality)
		if quality == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "quality parameter is required"})
		}

		// Parse query parameters
		page := parseQueryInt(c, "page", 1)
		if page < 1 {
			page = 1
		}
		pageSize := parseQueryInt(c, "page_size", 50)
		if pageSize < 1 || pageSize > 500 {
			pageSize = 50
		}
		mediaType := c.Query("media_type", "")

		// Build quality-based WHERE clause
		var whereClause string
		var heightRange string
		args := []interface{}{}

		switch quality {
		case "8K":
			whereClause = "WHERE li.height >= 4320"
			heightRange = "≥4320p"
		case "4K":
			whereClause = "WHERE li.height >= 2160"
			heightRange = "≥2160p"
		case "1080p":
			whereClause = "WHERE li.height >= 1080 AND li.height < 2160"
			heightRange = "1080p-2159p"
		case "720p":
			whereClause = "WHERE li.height >= 720 AND li.height < 1080"
			heightRange = "720p-1079p"
		case "SD":
			whereClause = "WHERE li.height >= 1 AND li.height < 720"
			heightRange = "1p-719p"
		case "Unknown", "Resolution Not Available":
			// Handle both legacy "Unknown" and current "Resolution Not Available"
			whereClause = "WHERE li.height IS NULL OR li.height = 0"
			heightRange = "No height data"
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid quality parameter. Must be: 8K, 4K, 1080p, 720p, SD, Unknown, or Resolution Not Available"})
		}

		// Add media type filter if specified
		if mediaType != "" {
			whereClause += " AND COALESCE(li.media_type, 'Unknown') = ?"
			args = append(args, mediaType)
		}

		// Exclude live/TV channel types
		whereClause += " AND COALESCE(li.media_type, 'Unknown') NOT IN ('TvChannel', 'LiveTv', 'Channel')"

		// Get total count
		countQuery := `
			SELECT COUNT(*)
			FROM library_item li
			` + whereClause

		var total int
		if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Get paginated items
		offset := (page - 1) * pageSize
		itemsQuery := `
			SELECT 
				li.id,
				COALESCE(li.name, 'Unknown') as name,
				COALESCE(li.media_type, 'Unknown') as media_type,
				li.height,
				li.width,
				COALESCE(li.video_codec, 'Unknown') as codec
			FROM library_item li
			` + whereClause + `
			ORDER BY li.name ASC
			LIMIT ? OFFSET ?`

		args = append(args, pageSize, offset)
		rows, err := db.Query(itemsQuery, args...)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var items []LibraryItemResponse
		for rows.Next() {
			var item LibraryItemResponse
			var height, width sql.NullInt64

			if err := rows.Scan(&item.ID, &item.Name, &item.MediaType, &height, &width, &item.Codec); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Convert sql.NullInt64 to *int
			if height.Valid {
				h := int(height.Int64)
				item.Height = &h
			}
			if width.Valid {
				w := int(width.Int64)
				item.Width = &w
			}

			items = append(items, item)
		}

		if err := rows.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(ItemsByQualityResponse{
			Items:       items,
			Total:       total,
			Quality:     quality,
			HeightRange: heightRange,
			Page:        page,
			PageSize:    pageSize,
		})
	}
}
