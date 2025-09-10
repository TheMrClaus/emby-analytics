package stats

import (
    "database/sql"

    "github.com/gofiber/fiber/v3"
)

type ItemsByGenreResponse struct {
    Items    []LibraryItemResponse `json:"items"`
    Total    int                   `json:"total"`
    Genre    string                `json:"genre"`
    Page     int                   `json:"page"`
    PageSize int                   `json:"page_size"`
}

// ItemsByGenre returns library items that contain the given genre token.
// Since genres are stored as comma-separated strings, we normalize and
// match on token boundaries using ("," || REPLACE(genres, ", ", ",") || ",") LIKE pattern.
func ItemsByGenre(db *sql.DB) fiber.Handler {
    return func(c fiber.Ctx) error {
        genre := c.Params("genre")
        if genre == "" {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "genre parameter is required"})
        }

        // Query params
        page := parseQueryInt(c, "page", 1)
        if page < 1 { page = 1 }
        pageSize := parseQueryInt(c, "page_size", 50)
        if pageSize < 1 || pageSize > 500 { pageSize = 50 }
        mediaType := c.Query("media_type", "")

        // Match by PRIMARY genre only to align with "Popular Genres" bubbles,
        // which compute counts based on the first token before the first comma.
        primary := "TRIM(CASE WHEN INSTR(genres, ',') > 0 THEN SUBSTR(genres, 1, INSTR(genres, ',') - 1) ELSE genres END)"
        where := "WHERE genres IS NOT NULL AND genres != '' AND (" + primary + ") = ?"
        args := []interface{}{genre}

        if mediaType != "" {
            where += " AND COALESCE(media_type, 'Unknown') = ?"
            args = append(args, mediaType)
        }

        // Exclude live TV-ish items
        where += " AND COALESCE(media_type, 'Unknown') NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')"

        // Count
        var total int
        if err := db.QueryRow("SELECT COUNT(*) FROM library_item " + where, args...).Scan(&total); err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
        }

        // Fetch page
        offset := (page - 1) * pageSize
        q := `
            SELECT id, COALESCE(name,'Unknown') AS name, COALESCE(media_type,'Unknown') AS media_type, height, width, COALESCE(video_codec,'Unknown') as codec
            FROM library_item
        ` + where + `
            ORDER BY name ASC
            LIMIT ? OFFSET ?`
        args = append(args, pageSize, offset)

        rows, err := db.Query(q, args...)
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
        }
        defer rows.Close()

        items := []LibraryItemResponse{}
        for rows.Next() {
            var item LibraryItemResponse
            var h, w sql.NullInt64
            if err := rows.Scan(&item.ID, &item.Name, &item.MediaType, &h, &w, &item.Codec); err != nil {
                return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
            }
            if h.Valid { hv := int(h.Int64); item.Height = &hv }
            if w.Valid { wv := int(w.Int64); item.Width = &wv }
            items = append(items, item)
        }
        if err := rows.Err(); err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(ItemsByGenreResponse{
            Items: items,
            Total: total,
            Genre: genre,
            Page: page,
            PageSize: pageSize,
        })
    }
}
