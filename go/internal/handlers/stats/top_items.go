package stats

import (
	"database/sql"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/queries"
	"emby-analytics/internal/tasks"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type TopItem struct {
	ItemID  string  `json:"item_id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display"`
}

func TopItems(db *sql.DB, em *emby.Client) fiber.Handler {
    return func(c fiber.Ctx) error {
		timeframe := c.Query("timeframe", "")
		if timeframe == "" {
			// Fallback to days parameter if timeframe not provided
			days := parseQueryInt(c, "days", 14)
			if days <= 0 {
				timeframe = "all-time"
			} else if days == 1 {
				timeframe = "1d"
			} else if days == 3 {
				timeframe = "3d"
			} else if days == 7 {
				timeframe = "7d"
			} else if days == 14 {
				timeframe = "14d"
			} else if days == 30 {
				timeframe = "30d"
			} else {
				timeframe = "30d" // Default for large day values
			}
		}
		if timeframe == "" {
			timeframe = "14d" // Final fallback
		}

		limit := parseQueryInt(c, "limit", 10)
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		days := parseTimeframeToDays(timeframe)
		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -days).Unix()

		if timeframe == "all-time" {
			winStart = 0
			winEnd = now.AddDate(100, 0, 0).Unix()
		}

        // 1. Get historical data (broad candidate set)
        historicalRows, err := queries.TopItemsByWatchSeconds(c, db, winStart, winEnd, 1000)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": "database query failed: " + err.Error()})
        }

		if err != nil || len(historicalRows) == 0 {
			// Fallback to counting sessions if intervals aren't populated
            rows, err := db.Query(`
        SELECT 
            li.id,
            li.name,
            li.media_type,
            COUNT(DISTINCT ps.id) * 0.5 as hours
        FROM library_item li
        LEFT JOIN play_sessions ps ON ps.item_id = li.id
        WHERE ps.started_at >= ? AND ps.started_at <= ?
          AND `+excludeLiveTvFilter()+`
        GROUP BY li.id, li.name, li.media_type
        ORDER BY hours DESC
        LIMIT ?
    `, winStart, winEnd, 1000)

			if err == nil {
				defer rows.Close()
				historicalRows = []queries.TopItemRow{}
				for rows.Next() {
					var r queries.TopItemRow
					if err := rows.Scan(&r.ItemID, &r.Name, &r.Type, &r.Hours); err == nil {
						r.Display = r.Name
						historicalRows = append(historicalRows, r)
					}
				}
			}
		}

        // 2. Build item details map and candidate set for precise duration calculation
        combinedHours := make(map[string]float64)
        itemDetails := make(map[string]TopItem)
        candidateIDs := make(map[string]struct{})
        for _, row := range historicalRows {
            // Exclude Live TV content from candidates
            if strings.EqualFold(row.Type, "TvChannel") || strings.EqualFold(row.Type, "LiveTv") || strings.EqualFold(row.Type, "Channel") || strings.EqualFold(row.Type, "TvProgram") {
                continue
            }
            itemDetails[row.ItemID] = TopItem{ItemID: row.ItemID, Name: row.Name, Type: row.Type}
            candidateIDs[row.ItemID] = struct{}{}
        }

		// 2.5. Always supplement from play_intervals to include items missing from library_item
		// This ensures currently playing or newly seen items appear even before metadata sync.
		{
            intervalRows, err := db.Query(`
                SELECT l.item_id, SUM(MIN(l.end_ts, ?) - MAX(l.start_ts, ?)) / 3600.0 as hours
                FROM play_intervals l
                JOIN library_item li ON li.id = l.item_id
                WHERE l.start_ts <= ? AND l.end_ts >= ?
                  AND `+excludeLiveTvFilter()+`
                GROUP BY l.item_id
                HAVING hours > 0
                ORDER BY hours DESC
                LIMIT ?
            `, winEnd, winStart, winEnd, winStart, 2000)

			if err == nil {
				defer intervalRows.Close()
				missingItemIDs := []string{}

				for intervalRows.Next() {
					var itemID string
					var hours float64
					if err := intervalRows.Scan(&itemID, &hours); err == nil {
                        // Track as candidate; exact computation performed below
                        candidateIDs[itemID] = struct{}{}

						// Ensure we have details; if missing in library_item, mark for fetch
						if _, ok := itemDetails[itemID]; !ok {
							var name, itemType string
							scanErr := db.QueryRow("SELECT name, media_type FROM library_item WHERE id = ?", itemID).Scan(&name, &itemType)
							if scanErr != nil {
								missingItemIDs = append(missingItemIDs, itemID)
								itemDetails[itemID] = TopItem{ItemID: itemID, Name: "Loading...", Type: "Unknown"}
							} else {
								itemDetails[itemID] = TopItem{ItemID: itemID, Name: name, Type: itemType}
							}
						}
					}
				}

				// Fetch missing items from Emby in batch for display + persist to library_item
				if len(missingItemIDs) > 0 && em != nil {
					if embyItems, fetchErr := em.ItemsByIDs(missingItemIDs); fetchErr == nil {
						for _, item := range embyItems {
							itemDetails[item.Id] = TopItem{ItemID: item.Id, Name: item.Name, Type: item.Type}
							_, _ = db.Exec(`
                                INSERT INTO library_item (id, server_id, item_id, name, media_type, created_at, updated_at)
                                VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
                                ON CONFLICT(id) DO UPDATE SET
                                    name = excluded.name,
                                    media_type = excluded.media_type,
                                    updated_at = CURRENT_TIMESTAMP
                            `, item.Id, item.Id, item.Id, item.Name, item.Type)
						}
					}
				}
			}
		}

        // 3. Compute exact, coalesced watch hours per candidate using per-session interval merging
        exactHours, err := computeExactItemHours(db, keys(candidateIDs), winStart, winEnd)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": "failed to compute exact hours: " + err.Error()})
        }
        for id, hrs := range exactHours {
            combinedHours[id] = hrs
        }

        // 4. Get live data and merge
        liveWatchTimes := tasks.GetLiveItemWatchTimes() // Returns seconds
        for itemID, seconds := range liveWatchTimes {
            // Determine type to allow exclusion of Live TV
            var name, itemType string
            if det, ok := itemDetails[itemID]; ok {
                name, itemType = det.Name, det.Type
            } else {
                _ = db.QueryRow("SELECT name, media_type FROM library_item WHERE id = ?", itemID).Scan(&name, &itemType)
            }
            if strings.EqualFold(itemType, "TvChannel") || strings.EqualFold(itemType, "LiveTv") || strings.EqualFold(itemType, "Channel") || strings.EqualFold(itemType, "TvProgram") {
                continue // Skip live TV from Top Items
            }

            combinedHours[itemID] += seconds / 3600.0

            // Ensure we have item details for display
            if _, ok := itemDetails[itemID]; !ok {
                if name == "" && em != nil {
                    if embyItems, fetchErr := em.ItemsByIDs([]string{itemID}); fetchErr == nil && len(embyItems) > 0 {
                        it := embyItems[0]
                        name = it.Name
                        itemType = it.Type
                        _, _ = db.Exec(`
                            INSERT INTO library_item (id, server_id, item_id, name, media_type, created_at, updated_at)
                            VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
                            ON CONFLICT(id) DO UPDATE SET
                                name = excluded.name,
                                media_type = excluded.media_type,
                                updated_at = CURRENT_TIMESTAMP
                        `, it.Id, it.Id, it.Id, it.Name, it.Type)
                    }
                }
                if name == "" { name = fmt.Sprintf("Unknown Item (%s)", itemID[:8]) }
                if itemType == "" { itemType = "Unknown" }
                itemDetails[itemID] = TopItem{ItemID: itemID, Name: name, Type: itemType}
            }
        }

        // 5. Convert map back to slice
        finalResult := make([]TopItem, 0, len(combinedHours))
        for itemID, hours := range combinedHours {
            details := itemDetails[itemID]
            finalResult = append(finalResult, TopItem{
                ItemID:  itemID,
                Name:    details.Name,
                Type:    details.Type,
                Hours:   hours,
                Display: details.Name, // Default display before enrichment
            })
        }

        // 6. Sort and limit
        sort.Slice(finalResult, func(i, j int) bool {
            return finalResult[i].Hours > finalResult[j].Hours
        })
        if len(finalResult) > limit {
            finalResult = finalResult[:limit]
        }

        // 7. Run your preserved enrichment logic on the final, combined list
        enrichItems(finalResult, em)

        return c.JSON(finalResult)
    }
}

// keys returns the set keys as a slice
func keys(m map[string]struct{}) []string {
    out := make([]string, 0, len(m))
    for k := range m {
        out = append(out, k)
    }
    return out
}

type interval struct { s int64; e int64 }

// computeExactItemHours merges overlapping intervals per session for the given item IDs and window.
// It returns total hours per item, clamped to [winStart, winEnd].
func computeExactItemHours(db *sql.DB, itemIDs []string, winStart, winEnd int64) (map[string]float64, error) {
    out := make(map[string]float64)
    if len(itemIDs) == 0 {
        return out, nil
    }

    // Fetch runtime per item (seconds) if available to cap per-session durations for Movies
    runtimeSec := make(map[string]float64)
    {
        placeholders := make([]string, len(itemIDs))
        args := make([]any, 0, len(itemIDs))
        for i, id := range itemIDs {
            placeholders[i] = "?"
            args = append(args, id)
        }
        q := fmt.Sprintf(`SELECT id, COALESCE(run_time_ticks,0) FROM library_item WHERE id IN (%s)`, strings.Join(placeholders, ","))
        rows, err := db.Query(q, args...)
        if err == nil {
            defer rows.Close()
            const ticksPerSecond = 10000000.0
            for rows.Next() {
                var id string
                var ticks int64
                if err := rows.Scan(&id, &ticks); err == nil && ticks > 0 {
                    runtimeSec[id] = float64(ticks) / ticksPerSecond
                }
            }
            _ = rows.Err()
        }
    }

    // Build IN clause placeholders
    placeholders := make([]string, len(itemIDs))
    args := make([]any, 0, len(itemIDs)+2)
    for i, id := range itemIDs {
        placeholders[i] = "?"
        args = append(args, id)
    }
    args = append(args, winEnd, winStart) // for clamp and filter

    query := fmt.Sprintf(`
        SELECT pi.item_id, ps.session_id, pi.start_ts, pi.end_ts
        FROM play_intervals pi
        JOIN play_sessions ps ON ps.id = pi.session_fk
        WHERE pi.item_id IN (%s)
          AND pi.start_ts <= ? AND pi.end_ts >= ?
        ORDER BY pi.item_id, ps.session_id, pi.start_ts, pi.end_ts
    `, strings.Join(placeholders, ","))

    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // Group by item_id + session_fk
    type key struct{ item string; sess string }
    groups := make(map[key][]interval)
    for rows.Next() {
        var item string
        var sess string
        var s, e int64
        if err := rows.Scan(&item, &sess, &s, &e); err != nil {
            return nil, err
        }
        // Clamp to window
        if s < winStart { s = winStart }
        if e > winEnd { e = winEnd }
        if e <= s { continue }
        k := key{item: item, sess: sess}
        groups[k] = append(groups[k], interval{s: s, e: e})
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }

    // Merge per group and accumulate seconds per item, with per-session cap based on runtime (if known)
    secs := make(map[string]int64)
    for k, ivs := range groups {
        if len(ivs) == 0 { continue }
        // Sort by start then end (already ordered by query but be safe)
        sort.Slice(ivs, func(i, j int) bool {
            if ivs[i].s == ivs[j].s { return ivs[i].e < ivs[j].e }
            return ivs[i].s < ivs[j].s
        })
        curS := ivs[0].s
        curE := ivs[0].e
        for i := 1; i < len(ivs); i++ {
            if ivs[i].s <= curE { // overlap or touch
                if ivs[i].e > curE { curE = ivs[i].e }
            } else {
                // add merged segment for this session
                merged := (curE - curS)
                // Apply per-session cap using runtime if available (allow 1.5x slack)
                if rt, ok := runtimeSec[k.item]; ok && rt > 0 {
                    capSeconds := int64(rt * 1.5)
                    if merged > capSeconds { merged = capSeconds }
                }
                secs[k.item] += merged
                curS, curE = ivs[i].s, ivs[i].e
            }
        }
        // close last segment
        merged := (curE - curS)
        if rt, ok := runtimeSec[k.item]; ok && rt > 0 {
            capSeconds := int64(rt * 1.5)
            if merged > capSeconds { merged = capSeconds }
        }
        secs[k.item] += merged
    }

    for item, s := range secs {
        out[item] = float64(s) / 3600.0
    }
    return out, nil
}

// Your original enrichment logic, now in a helper function for clarity.
func enrichItems(items []TopItem, em *emby.Client) {
	allEnrichIDs := make([]string, 0)
	for _, item := range items {
		if strings.EqualFold(item.Type, "Episode") || item.Name == "Unknown" || item.Type == "Unknown" {
			allEnrichIDs = append(allEnrichIDs, item.ItemID)
		}
	}

	if len(allEnrichIDs) > 0 && em != nil {
		if embyItems, err := em.ItemsByIDs(allEnrichIDs); err == nil {
			embyMap := make(map[string]*emby.EmbyItem)
			for i := range embyItems {
				embyMap[embyItems[i].Id] = &embyItems[i]
			}
			for i := range items {
				item := &items[i]
				if it, ok := embyMap[item.ItemID]; ok {
					if strings.EqualFold(item.Type, "Episode") || it.Type == "Episode" {
						// For episodes, always use the canonical episode title from Emby
						if it.Name != "" {
							item.Name = it.Name
						}
						if it.SeriesName != "" {
							epcode := ""
							if it.ParentIndexNumber != nil && it.IndexNumber != nil {
								epcode = fmt.Sprintf("S%02dE%02d", *it.ParentIndexNumber, *it.IndexNumber)
							}
							if epcode != "" && item.Name != "" {
								item.Display = fmt.Sprintf("%s - %s (%s)", it.SeriesName, item.Name, epcode)
							} else if item.Name != "" {
								item.Display = fmt.Sprintf("%s - %s", it.SeriesName, item.Name)
							} else {
								item.Display = it.SeriesName
							}
							// Keep the type as Episode for clarity
							item.Type = "Episode"
						} else {
							// No series name: just show the episode name
							item.Display = item.Name
							item.Type = "Episode"
						}
					} else {
						if it.Name != "" && (item.Name == "Unknown" || item.Name == "") {
							item.Name = it.Name
							item.Display = it.Name
						}
						if it.Type != "" && (item.Type == "Unknown" || item.Type == "") {
							item.Type = it.Type
						}
					}
				} else if item.Name == "Unknown" || item.Type == "Unknown" {
					item.Name = fmt.Sprintf("Deleted Item (%s)", item.ItemID[:8])
					item.Display = fmt.Sprintf("Deleted Item (%s)", item.ItemID[:8])
					item.Type = "Deleted"
				}
			}
		}
	}
}
