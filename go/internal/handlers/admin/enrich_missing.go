package admin

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/media"
)

// EnrichMissingItems scans recent play_sessions for items missing names in library_item and enriches them via the appropriate server client.
// POST /admin/enrich/missing-items?days=30&limit=200
func EnrichMissingItems(db *sql.DB, mgr *media.MultiServerManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		if mgr == nil {
			return c.Status(503).JSON(fiber.Map{"error": "multi-server not initialized"})
		}
		days := parseIntEnrich(c.Query("days", "30"), 30)
		limit := parseIntEnrich(c.Query("limit", "200"), 200)
		if limit <= 0 {
			limit = 200
		}
		// Find item_ids with missing/placeholder names in the recent window
		q := `
            SELECT ps.item_id, ps.server_id
            FROM play_sessions ps
            LEFT JOIN library_item li ON li.id = ps.item_id
            WHERE ps.started_at >= strftime('%s','now','-` + fmt.Sprintf("%d", days) + ` day')
              AND (
                    li.name IS NULL OR li.name = ''
                 OR li.name LIKE 'Deleted Item (%)'
                 OR li.name LIKE 'Unknown Item (%)'
                 OR li.media_type IS NULL OR li.media_type = ''
              )
            GROUP BY ps.item_id, ps.server_id
            ORDER BY MAX(ps.started_at) DESC
            LIMIT ?
        `
		rows, err := db.Query(q, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()
		type pair struct{ id, sid string }
		toEnrich := make([]pair, 0)
		for rows.Next() {
			var id, sid string
			if err := rows.Scan(&id, &sid); err == nil && id != "" && sid != "" {
				toEnrich = append(toEnrich, pair{id: id, sid: sid})
			}
		}

		// Batch per server
		byServer := make(map[string][]string)
		for _, p := range toEnrich {
			byServer[p.sid] = append(byServer[p.sid], p.id)
		}
		updated := 0
		for sid, ids := range byServer {
			client, ok := mgr.GetClient(sid)
			if !ok || client == nil || len(ids) == 0 {
				continue
			}
			items, err := client.ItemsByIDs(ids)
			if err != nil {
				continue
			}
			for _, it := range items {
				name := it.Name
				if name == "" {
					name = fmt.Sprintf("Unknown Item (%s)", it.ID[:min(8, len(it.ID))])
				}
				mtype := it.Type
				if mtype == "" {
					mtype = "Unknown"
				}
				_, _ = db.Exec(`
                    INSERT INTO library_item (id, server_id, name, media_type, updated_at)
                    VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
                    ON CONFLICT(id) DO UPDATE SET
                        name = CASE WHEN excluded.name <> '' THEN excluded.name ELSE name END,
                        media_type = CASE WHEN excluded.media_type <> '' THEN excluded.media_type ELSE media_type END,
                        updated_at = CURRENT_TIMESTAMP
                `, it.ID, sid, name, mtype)
				updated++
			}
		}
		return c.JSON(fiber.Map{"enriched": updated, "servers": len(byServer)})
	}
}

func parseIntEnrich(s string, def int) int {
	var v int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	if err != nil {
		return def
	}
	return v
}
