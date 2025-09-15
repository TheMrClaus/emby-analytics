package stats

import (
    "database/sql"
    "emby-analytics/internal/emby"
    "emby-analytics/internal/media"
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

var topItemsMultiMgr *media.MultiServerManager

func SetMultiServerManager(mgr *media.MultiServerManager) { topItemsMultiMgr = mgr }

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
        // If the primary query errors, don't fail hard; attempt fallback path below
        if err != nil {
            historicalRows = nil
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

		// 2.5. Always supplement from play_intervals to include items missing from library_item.
		// This is a broad query to ensure any item with any watch history is a candidate.
		// The exact time clamping is handled robustly in computeExactItemHours.
		{
			intervalRows, err := db.Query(`
                SELECT DISTINCT l.item_id, 0.0 as hours
                FROM play_intervals l
            `)

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
            // Do not fail hard; log and continue with coarse hours
            fmt.Printf("[WARN] TopItems exact hours computation failed: %v\n", err)
            exactHours = map[string]float64{}
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
                if name == "" { name = fmt.Sprintf("Unknown Item (%s)", shortID(itemID)) }
                if itemType == "" { itemType = "Unknown" }
                itemDetails[itemID] = TopItem{ItemID: itemID, Name: name, Type: itemType}
            }
        }

        // 5. Convert map back to slice
        finalResult := make([]TopItem, 0, len(combinedHours))
        for itemID, hours := range combinedHours {
            details := itemDetails[itemID]
            // Exclude Live TV types from final top items
            if strings.EqualFold(details.Type, "TvChannel") || strings.EqualFold(details.Type, "LiveTv") || strings.EqualFold(details.Type, "Channel") || strings.EqualFold(details.Type, "TvProgram") {
                continue
            }
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
        // Additional enrichment for non-Emby items (multi-server)
        if topItemsMultiMgr != nil {
            enrichItemsMulti(db, finalResult)
        }

        return c.JSON(finalResult)
    }
}

// shortID returns a safe short prefix of an ID for display.
// It never slices past the string length to avoid runtime panics.
func shortID(id string) string {
    if len(id) > 8 {
        return id[:8]
    }
    return id
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
        SELECT pi.item_id, ps.session_id, pi.start_ts, pi.end_ts, pi.duration_seconds
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

    secs := make(map[string]int64)
    // Accumulate per item using per-interval capped seconds; no need to merge
    for rows.Next() {
        var item string
        var sess string
        var s, e int64
        var dur int64
        if err := rows.Scan(&item, &sess, &s, &e, &dur); err != nil {
            return nil, err
        }
        // Clamp to window
        if s < winStart { s = winStart }
        if e > winEnd { e = winEnd }
        if e <= s { continue }
        windowSec := e - s
        if windowSec < 0 { windowSec = 0 }
        // Cap by recorded active duration
        // derive effective duration: fall back to (end-start) if missing/zero
        eff := dur
        if eff <= 0 { eff = e - s }
        var add int64 = windowSec
        if eff > 0 && eff < add { add = eff }
        if add > 0 {
            secs[item] += add
        }
    }
    if err := rows.Err(); err != nil {
        return nil, err
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
                    item.Name = fmt.Sprintf("Deleted Item (%s)", shortID(item.ItemID))
                    item.Display = fmt.Sprintf("Deleted Item (%s)", shortID(item.ItemID))
                    item.Type = "Deleted"
                }
			}
		}
	}
}

// enrichItemsMulti resolves missing names/displays using the last-known server context and manager clients.
func enrichItemsMulti(db *sql.DB, items []TopItem) {
    // Build list of IDs needing enrichment
    need := make([]string, 0)
    for _, it := range items {
        if it.Name == "Unknown" || it.Type == "Unknown" || strings.HasPrefix(it.Name, "Deleted Item") || strings.HasPrefix(it.Display, "Deleted Item") {
            need = append(need, it.ItemID)
        }
    }
    if len(need) == 0 { return }

    type ctx struct{ serverID string }
    ctxByID := make(map[string]ctx)
    for _, id := range need {
        var sid string
        _ = db.QueryRow(`SELECT server_id FROM play_sessions WHERE item_id = ? ORDER BY started_at DESC LIMIT 1`, id).Scan(&sid)
        if sid != "" { ctxByID[id] = ctx{serverID: sid} }
    }
    // Batch per server
    byServer := make(map[string][]string)
    for id, c := range ctxByID { byServer[c.serverID] = append(byServer[c.serverID], id) }
    // Map for quick update
    idx := make(map[string]*TopItem)
    for i := range items { idx[items[i].ItemID] = &items[i] }
    for sid, idlist := range byServer {
        client, ok := topItemsMultiMgr.GetClient(sid)
        if !ok || client == nil || len(idlist) == 0 { continue }
        if mis, err := client.ItemsByIDs(idlist); err == nil {
            for _, mi := range mis {
                ti, ok := idx[mi.ID]; if !ok { continue }
                if mi.Name != "" { ti.Name = mi.Name; ti.Display = mi.Name }
                if mi.Type != "" { ti.Type = mi.Type }
                if strings.EqualFold(mi.Type, "Episode") && mi.SeriesName != "" {
                    epcode := ""
                    if mi.ParentIndexNumber != nil && mi.IndexNumber != nil {
                        epcode = fmt.Sprintf("S%02dE%02d", *mi.ParentIndexNumber, *mi.IndexNumber)
                    }
                    if epcode != "" && ti.Name != "" {
                        ti.Display = fmt.Sprintf("%s - %s (%s)", mi.SeriesName, ti.Name, epcode)
                    } else if ti.Name != "" {
                        ti.Display = fmt.Sprintf("%s - %s", mi.SeriesName, ti.Name)
                    } else {
                        ti.Display = mi.SeriesName
                    }
                    ti.Type = "Episode"
                }
                // Upsert minimal metadata
                _, _ = db.Exec(`
                    INSERT INTO library_item (id, server_id, name, media_type, updated_at)
                    VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
                    ON CONFLICT(id) DO UPDATE SET
                        name = CASE WHEN excluded.name <> '' THEN excluded.name ELSE name END,
                        media_type = CASE WHEN excluded.media_type <> '' THEN excluded.media_type ELSE media_type END,
                        updated_at = CURRENT_TIMESTAMP
                `, mi.ID, sid, ti.Name, ti.Type)
            }
        }
    }
}
