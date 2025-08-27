package stats

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

type TopItem struct {
	ItemID  string  `json:"item_id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display"` // Add enriched display field
}

// /stats/top/items?days=30&limit=10
func TopItems(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get timeframe parameter: all-time, 30d, 14d, 7d, 3d, 1d
		timeframe := c.Query("timeframe", "14d")
		limit := parseQueryInt(c, "limit", 10)

		if limit <= 0 || limit > 100 {
			limit = 10
		}

		var rows *sql.Rows
		var err error

		// Handle "all-time" vs time-windowed differently
		if timeframe == "all-time" {
			// For all-time, use a simple heuristic since we don't have item-level lifetime data
			// This is less accurate but gives a broad view of popular content
			rows, err = db.Query(`
				SELECT 
					li.id, 
					COALESCE(li.name, 'Unknown') as name, 
					COALESCE(li.type, 'Unknown') as type,
					COUNT(DISTINCT pe.user_id) * 
					CASE 
						WHEN li.type = 'Movie' THEN 2.0    -- Average movie length
						WHEN li.type = 'Episode' THEN 0.75 -- Average episode length  
						ELSE 1.2
					END AS hours
				FROM play_event pe
				LEFT JOIN library_item li ON li.id = pe.item_id
				WHERE pe.item_id != '' AND pe.pos_ms > 300000  -- 5+ minute sessions
					AND li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
				GROUP BY li.id, li.name, li.type
				ORDER BY hours DESC
				LIMIT ?;
			`, limit)
		} else {
			// Handle time-windowed queries with accurate position-based calculation
			days := parseTimeframeToDays(timeframe)
			if days <= 0 {
				days = 14 // fallback
			}

			fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

			rows, err = db.Query(`
				SELECT 
					li.id, 
					COALESCE(li.name, 'Unknown') as name, 
					COALESCE(li.type, 'Unknown') as type,
					SUM(max_pos_ms) / 3600000.0 AS hours
				FROM (
					-- Get the max watch position for each user+item combination within the time window
					SELECT
						user_id,
						item_id,
						MAX(pos_ms) as max_pos_ms
					FROM play_event
					WHERE ts >= ? AND item_id != '' AND pos_ms > 60000  -- 1+ minute sessions
					GROUP BY user_id, item_id
				) AS user_item_max
				LEFT JOIN library_item li ON li.id = user_item_max.item_id
				WHERE li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
				GROUP BY li.id, li.name, li.type
				HAVING SUM(max_pos_ms) > 600000  -- At least 10 minutes total
				ORDER BY hours DESC
				LIMIT ?;
			`, fromMs, limit)
		}

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		items := make(map[string]TopItem)
		episodeIDs := make([]string, 0)

		for rows.Next() {
			var ti TopItem
			if err := rows.Scan(&ti.ItemID, &ti.Name, &ti.Type, &ti.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Set default display to name
			ti.Display = ti.Name
			items[ti.ItemID] = ti

			// Collect episode IDs for enrichment
			if strings.EqualFold(ti.Type, "Episode") {
				episodeIDs = append(episodeIDs, ti.ItemID)
			}
		}

		// Enrich Episodes via Emby API (same as before)
		if len(episodeIDs) > 0 && em != nil {
			if embyItems, err := em.ItemsByIDs(episodeIDs); err == nil {
				for _, it := range embyItems {
					if rec, ok := items[it.Id]; ok {
						// Build enhanced display name (same logic as before)
						name := rec.Name
						if (name == "" || name == it.Name) && it.Name != "" {
							name = it.Name
							rec.Name = name
						}

						season := it.ParentIndexNumber
						ep := it.IndexNumber
						series := it.SeriesName
						epname := name

						if series == "" {
							rec.Display = epname
						} else {
							epcode := ""
							if season != nil && ep != nil {
								epcode = fmt.Sprintf("S%02dE%02d", *season, *ep)
							}
							if epcode != "" && epname != "" {
								rec.Display = fmt.Sprintf("%s - %s (%s)", series, epname, epcode)
							} else if epname != "" {
								rec.Display = fmt.Sprintf("%s - %s", series, epname)
							} else {
								rec.Display = series
							}
							rec.Type = "Series"
						}
						items[it.Id] = rec
					}
				}
			}
		}

		// Convert map back to slice, preserving order by hours
		out := make([]TopItem, 0, len(items))

		// Re-query to preserve the original order since we used a map
		if timeframe == "all-time" {
			rows, err = db.Query(`
				SELECT 
					li.id, 
					COUNT(DISTINCT pe.user_id) * 
					CASE 
						WHEN li.type = 'Movie' THEN 2.0
						WHEN li.type = 'Episode' THEN 0.75  
						ELSE 1.2
					END AS hours
				FROM play_event pe
				LEFT JOIN library_item li ON li.id = pe.item_id
				WHERE pe.item_id != '' AND pe.pos_ms > 300000
					AND li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
				GROUP BY li.id, li.name, li.type
				ORDER BY hours DESC
				LIMIT ?;
			`, limit)
		} else {
			days := parseTimeframeToDays(timeframe)
			fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

			rows, err = db.Query(`
				SELECT 
					li.id, 
					SUM(max_pos_ms) / 3600000.0 AS hours
				FROM (
					SELECT
						user_id,
						item_id,
						MAX(pos_ms) as max_pos_ms
					FROM play_event
					WHERE ts >= ? AND item_id != '' AND pos_ms > 60000
					GROUP BY user_id, item_id
				) AS user_item_max
				LEFT JOIN library_item li ON li.id = user_item_max.item_id
				WHERE li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
				GROUP BY li.id, li.name, li.type
				HAVING SUM(max_pos_ms) > 600000
				ORDER BY hours DESC
				LIMIT ?;
			`, fromMs, limit)
		}

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		for rows.Next() {
			var itemID string
			var hours float64
			if err := rows.Scan(&itemID, &hours); err != nil {
				continue
			}
			if item, ok := items[itemID]; ok {
				out = append(out, item)
			}
		}

		return c.JSON(out)
	}
}
