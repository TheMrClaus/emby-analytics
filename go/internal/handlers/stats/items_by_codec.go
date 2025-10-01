package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

// LibraryItemResponse represents a library item for API responses
type LibraryItemResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Height    *int   `json:"height"`
	Width     *int   `json:"width"`
	Codec     string `json:"codec"`
}

// ItemsByCodecResponse holds the response structure
type ItemsByCodecResponse struct {
	Items    []LibraryItemResponse `json:"items"`
	Total    int                   `json:"total"`
	Codec    string                `json:"codec"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// ItemsByCodec returns all library items for a specific codec with pagination
func ItemsByCodec(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		codec := c.Params("codec")
		if codec == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "codec parameter is required"})
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

		// Build query conditions
		whereClause := "WHERE COALESCE(li.video_codec, 'Unknown') = ?"
		args := []interface{}{codec}

		if mediaType != "" {
			whereClause += " AND COALESCE(li.media_type, 'Unknown') = ?"
			args = append(args, mediaType)
		}

		// Exclude live/TV channel types
		whereClause += " AND COALESCE(li.media_type, 'Unknown') NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')"

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

		return c.JSON(ItemsByCodecResponse{
			Items:    items,
			Total:    total,
			Codec:    codec,
			Page:     page,
			PageSize: pageSize,
		})
	}
}
