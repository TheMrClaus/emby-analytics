package admin

import (
    "database/sql"
    "strings"

    "emby-analytics/internal/emby"
    "github.com/gofiber/fiber/v3"
)

// CleanupMissingItems scans library_item for IDs that no longer exist in Emby.
// GET  /admin/cleanup/missing-items        -> dry-run summary
// POST /admin/cleanup/missing-items        -> delete rows with no play_intervals
// Optional: ?limit=1000 (batch size)
func CleanupMissingItems(db *sql.DB, em *emby.Client) fiber.Handler {
    return func(c fiber.Ctx) error {
        if em == nil {
            return c.Status(500).JSON(fiber.Map{"error": "Emby client not configured"})
        }
        limit := 1000
        if v := c.Query("limit", ""); v != "" {
            if n := parseInt(v); n > 0 { limit = n }
        }
        apply := string(c.Request().Header.Method()) == fiber.MethodPost

        // Collect candidate IDs
        rows, err := db.Query(`SELECT id FROM library_item LIMIT ?`, limit)
        if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
        defer rows.Close()
        ids := []string{}
        for rows.Next() { var id string; if err := rows.Scan(&id); err == nil { ids = append(ids, id) } }
        if len(ids) == 0 { return c.JSON(fiber.Map{"checked": 0, "missing": 0, "deleted": 0}) }

        // Check existence in Emby in chunks
        chunk := 50
        found := make(map[string]struct{}, len(ids))
        for i := 0; i < len(ids); i += chunk {
            end := i + chunk; if end > len(ids) { end = len(ids) }
            part := ids[i:end]
            items, err := em.ItemsByIDs(part)
            if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
            for _, it := range items { found[strings.TrimSpace(it.Id)] = struct{}{} }
        }

        // Determine missing and classify by intervals presence
        missing := []string{}
        missingNoIntervals := []string{}
        for _, id := range ids {
            if _, ok := found[id]; ok { continue }
            missing = append(missing, id)
            var exists int
            _ = db.QueryRow(`SELECT 1 FROM play_intervals WHERE item_id = ? LIMIT 1`, id).Scan(&exists)
            if exists == 0 { missingNoIntervals = append(missingNoIntervals, id) }
        }

        deleted := 0
        if apply && len(missingNoIntervals) > 0 {
            // Delete safe ones (no intervals)
            // Use simple loop to avoid SQLite var limits
            for _, id := range missingNoIntervals {
                if _, err := db.Exec(`DELETE FROM library_item WHERE id = ?`, id); err == nil { deleted++ }
            }
        }

        return c.JSON(fiber.Map{
            "checked": len(ids),
            "missing": len(missing),
            "missing_no_intervals": len(missingNoIntervals),
            "deleted": deleted,
            "applied": apply,
        })
    }
}

func parseInt(s string) int {
    n := 0
    for _, ch := range s { if ch < '0' || ch > '9' { return n }; n = n*10 + int(ch-'0') }
    return n
}

