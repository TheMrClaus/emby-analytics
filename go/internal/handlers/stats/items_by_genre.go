package stats

import (
    "database/sql"
    "strings"

    "github.com/gofiber/fiber/v3"
)

type ItemWithGenresResponse struct {
    ID        string   `json:"id"`
    Name      string   `json:"name"`
    MediaType string   `json:"media_type"`
    Height    *int     `json:"height"`
    Width     *int     `json:"width"`
    Codec     string   `json:"codec"`
    Genres    []string `json:"genres"`
}

type ItemsByGenreResponse struct {
    Items    []ItemWithGenresResponse `json:"items"`
    Total    int                      `json:"total"`
    Genre    string                   `json:"genre"`
    Page     int                      `json:"page"`
    PageSize int                      `json:"page_size"`
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

        // Match any genre token (case‑insensitive, token boundary)
        where := "WHERE genres IS NOT NULL AND genres != '' AND INSTR(LOWER(',' || REPLACE(genres, ', ', ',') || ','), LOWER(',' || ? || ',')) > 0"
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
            SELECT id, COALESCE(name,'Unknown') AS name, COALESCE(media_type,'Unknown') AS media_type, height, width, COALESCE(video_codec,'Unknown') as codec, genres
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

        items := []ItemWithGenresResponse{}
        for rows.Next() {
            var item ItemWithGenresResponse
            var h, w sql.NullInt64
            var genres sql.NullString
            if err := rows.Scan(&item.ID, &item.Name, &item.MediaType, &h, &w, &item.Codec, &genres); err != nil {
                return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
            }
            if h.Valid { hv := int(h.Int64); item.Height = &hv }
            if w.Valid { wv := int(w.Int64); item.Width = &wv }
            if genres.Valid {
                g := strings.ReplaceAll(genres.String, ", ", ",")
                parts := strings.Split(g, ",")
                out := make([]string, 0, len(parts))
                seen := map[string]struct{}{}
                for _, p := range parts {
                    t := strings.TrimSpace(p)
                    if t == "" { continue }
                    lt := strings.ToLower(t)
                    if _, ok := seen[lt]; ok { continue }
                    seen[lt] = struct{}{}
                    out = append(out, t)
                }
                item.Genres = out
            }
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
