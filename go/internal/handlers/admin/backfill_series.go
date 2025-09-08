package admin

import (
    "database/sql"
    "strings"

    "emby-analytics/internal/emby"
    "github.com/gofiber/fiber/v3"
)

// BackfillSeries populates series_id/series_name for episodes missing linkage
// GET: dry-run summary; POST: apply updates.
func BackfillSeries(db *sql.DB, em *emby.Client) fiber.Handler {
    return func(c fiber.Ctx) error {
        if em == nil {
            return c.Status(500).JSON(fiber.Map{"error": "Emby client not configured"})
        }
        method := string(c.Request().Header.Method())
        apply := method == fiber.MethodPost

        // collect up to N episodes missing series_id
        rows, err := db.Query(`SELECT id FROM library_item WHERE media_type='Episode' AND (series_id IS NULL OR series_id='') LIMIT 500`)
        if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
        defer rows.Close()
        ids := []string{}
        for rows.Next() {
            var id string
            if err := rows.Scan(&id); err == nil { ids = append(ids, id) }
        }
        if len(ids) == 0 {
            return c.JSON(fiber.Map{"updated": 0, "pending": 0})
        }

        items, err := em.ItemsByIDs(ids)
        if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }

        updated := 0
        for _, it := range items {
            if !strings.EqualFold(it.Type, "Episode") { continue }
            sid := strings.TrimSpace(it.SeriesId)
            sname := strings.TrimSpace(it.SeriesName)
            if sid == "" && sname != "" {
                if seriesID, _ := em.FindSeriesIDByName(sname); seriesID != "" { sid = seriesID }
            }
            if sid == "" && sname == "" { continue }
            if apply {
                if _, err := db.Exec(`UPDATE library_item SET series_id = COALESCE(?, series_id), series_name = COALESCE(?, series_name) WHERE id = ?`, nullIfEmpty(sid), nullIfEmpty(sname), it.Id); err == nil {
                    updated++
                }
            } else {
                updated++ // report would update
            }
        }
        return c.JSON(fiber.Map{"updated": updated, "pending": len(ids), "applied": apply})
    }
}

