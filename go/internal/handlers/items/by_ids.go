package items

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
	"emby-analytics/internal/media"
)

type ItemRow struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

// GET /items/by-ids?ids=a,b,c
func ByIDs(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		raw := c.Query("ids", "")
		if strings.TrimSpace(raw) == "" {
			return c.JSON([]ItemRow{}) // empty list, not null
		}
		ids := make([]string, 0)
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				ids = append(ids, p)
			}
		}
		if len(ids) == 0 {
			return c.JSON([]ItemRow{})
		}

		// 1) Get what we already have in SQLite
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1] // drop trailing comma

		args := make([]any, len(ids))
		for i, v := range ids {
			args[i] = v
		}

		rows, err := db.Query(
			`SELECT id, name, media_type FROM library_item WHERE id IN (`+placeholders+`)`, args...,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		base := make(map[string]ItemRow, len(ids))
		for rows.Next() {
			var r ItemRow
			if err := rows.Scan(&r.ID, &r.Name, &r.Type); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			// Debug logging
			log.Printf("DB item %s: name='%s', type='%s'", r.ID, r.Name, r.Type)

			// Default display = name for non-episodes (or unknown)
			r.Display = r.Name
			base[r.ID] = r
		}

		// 2) Enrich Episodes via Emby API to build nice display strings
		episodeIDs := make([]string, 0)
		for _, id := range ids {
			if rec, ok := base[id]; ok && strings.EqualFold(rec.Type, "Episode") {
				episodeIDs = append(episodeIDs, id)
			}
		}
		if len(episodeIDs) > 0 {
			if em != nil {
				log.Printf("Enriching %d episodes: %v", len(episodeIDs), episodeIDs)
				if items, err := em.ItemsByIDs(episodeIDs); err == nil {
					log.Printf("Emby API returned %d items", len(items))
					for _, it := range items {
						log.Printf("Emby item %s: name='%s', type='%s', series='%s'",
							it.Id, it.Name, it.Type, it.SeriesName)

						rec := base[it.Id]
						// Prefer Emby item name for episode title to avoid duplicating pre-enriched strings
						if it.Name != "" {
							rec.Name = it.Name
						}
						// Build display from Emby fields (SeriesName + item.Name + epcode)
						season := it.ParentIndexNumber
						ep := it.IndexNumber
						series := it.SeriesName
						epname := it.Name

						// Handle cases where we have partial data
						if series == "" {
							// Try to get series name from the episode's parent
							rec.Display = epname
							rec.Type = "Episode"
						} else {
							// We have series info, build full display
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
							// Keep Episode type
							rec.Type = "Episode"
						}
						base[it.Id] = rec
					}
				} else {
					log.Printf("Emby API error for episodes: %v", err)
				}
			} else {
				log.Printf("Emby client is nil, cannot enrich episodes")
			}
		}

		// 3) Build output in the same order as requested
		out := make([]ItemRow, 0, len(ids))
		for _, id := range ids {
			if r, ok := base[id]; ok {
				// Ensure we have at least basic info
				if r.Name == "" && r.Type == "" {
					// Item exists in DB but has no data - try to get from Emby directly
					log.Printf("Item %s has no name/type, attempting direct Emby lookup", id)
					if em != nil {
						if items, err := em.ItemsByIDs([]string{id}); err == nil && len(items) > 0 {
							item := items[0]
							r.Name = item.Name
							r.Type = item.Type
							if strings.EqualFold(item.Type, "Episode") && item.SeriesName != "" {
								// Build episode display
								epcode := ""
								if item.ParentIndexNumber != nil && item.IndexNumber != nil {
									epcode = fmt.Sprintf("S%02dE%02d", *item.ParentIndexNumber, *item.IndexNumber)
								}
								if epcode != "" && item.Name != "" {
									r.Display = fmt.Sprintf("%s - %s (%s)", item.SeriesName, item.Name, epcode)
								} else {
									r.Display = fmt.Sprintf("%s - %s", item.SeriesName, item.Name)
								}
								// Keep Episode type
								r.Type = "Episode"
							} else {
								r.Display = r.Name
							}
							log.Printf("Direct lookup success for %s: name='%s', display='%s'", id, r.Name, r.Display)
						} else {
							log.Printf("Direct Emby lookup failed for %s: %v", id, err)
						}
					}
				}

				// Final fallbacks
				if r.Display == "" {
					if r.Name != "" {
						r.Display = r.Name
					} else {
						r.Display = fmt.Sprintf("Unknown Item (%s)", id)
					}
				}
				if r.Type == "" {
					r.Type = "Unknown"
				}

				out = append(out, r)
			} else {
				// Unknown ID: not in database at all. Best-effort lookup via Emby.
				log.Printf("Item %s not found in database; attempting Emby lookup", id)
				if em != nil {
					if items, err := em.ItemsByIDs([]string{id}); err == nil && len(items) > 0 {
						it := items[0]
						rec := ItemRow{ID: it.Id, Name: it.Name, Type: it.Type}
						// Build display for episodes; otherwise, use name
						if strings.EqualFold(it.Type, "Episode") && it.SeriesName != "" {
							epcode := ""
							if it.ParentIndexNumber != nil && it.IndexNumber != nil {
								epcode = fmt.Sprintf("S%02dE%02d", *it.ParentIndexNumber, *it.IndexNumber)
							}
							if epcode != "" && it.Name != "" {
								rec.Display = fmt.Sprintf("%s - %s (%s)", it.SeriesName, it.Name, epcode)
							} else if it.Name != "" {
								rec.Display = fmt.Sprintf("%s - %s", it.SeriesName, it.Name)
							} else {
								rec.Display = it.SeriesName
							}
						} else {
							if rec.Name != "" {
								rec.Display = rec.Name
							} else {
								rec.Display = fmt.Sprintf("Unknown Item (%s)", id)
							}
						}
						out = append(out, rec)
						continue
					}
				}
				out = append(out, ItemRow{
					ID:      id,
					Name:    fmt.Sprintf("Missing Item (%s)", id),
					Type:    "Unknown",
					Display: fmt.Sprintf("Missing Item (%s)", id),
				})
			}
		}
		return c.JSON(out)
	}
}

// ByIDsMS uses the MultiServerManager to enrich items by consulting the most recent server context per item.
func ByIDsMS(db *sql.DB, mgr *media.MultiServerManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		raw := c.Query("ids", "")
		if strings.TrimSpace(raw) == "" {
			return c.JSON([]ItemRow{})
		}
		parts := strings.Split(raw, ",")
		ids := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				ids = append(ids, p)
			}
		}
		if len(ids) == 0 {
			return c.JSON([]ItemRow{})
		}

		// Base rows from DB
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(ids))
		for i, v := range ids {
			args[i] = v
		}
		rows, err := db.Query(`SELECT id, name, media_type FROM library_item WHERE id IN (`+placeholders+`)`, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()
		base := make(map[string]ItemRow, len(ids))
		for rows.Next() {
			var r ItemRow
			if err := rows.Scan(&r.ID, &r.Name, &r.Type); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			r.Display = r.Name
			base[r.ID] = r
		}

		// Resolve server context for missing/placeholder items
		type ctx struct {
			serverID   string
			serverType media.ServerType
		}
		ctxByID := make(map[string]ctx)
		for _, id := range ids {
			r, ok := base[id]
			if ok && r.Name != "" && r.Name != "Unknown" && r.Name != "Missing" {
				continue
			}
			var sid, stype string
			_ = db.QueryRow(`SELECT server_id, server_type FROM play_sessions WHERE item_id = ? ORDER BY started_at DESC LIMIT 1`, id).Scan(&sid, &stype)
			if sid != "" {
				ctxByID[id] = ctx{serverID: sid, serverType: media.ServerType(stype)}
			}
		}

		// Batch per server client
		batch := make(map[string][]string)
		for id, cxt := range ctxByID {
			batch[cxt.serverID] = append(batch[cxt.serverID], id)
		}
		for serverID, idlist := range batch {
			client, ok := mgr.GetClient(serverID)
			if !ok || client == nil || len(idlist) == 0 {
				continue
			}
			if items, err := client.ItemsByIDs(idlist); err == nil {
				for _, it := range items {
					rec := base[it.ID]
					if it.Name != "" {
						rec.Name = it.Name
					}
					if it.Type != "" {
						rec.Type = it.Type
					}
					// Display for episodes
					if strings.EqualFold(it.Type, "Episode") && it.SeriesName != "" {
						epcode := ""
						if it.ParentIndexNumber != nil && it.IndexNumber != nil {
							epcode = fmt.Sprintf("S%02dE%02d", *it.ParentIndexNumber, *it.IndexNumber)
						}
						if epcode != "" && rec.Name != "" {
							rec.Display = fmt.Sprintf("%s - %s (%s)", it.SeriesName, rec.Name, epcode)
						} else if rec.Name != "" {
							rec.Display = fmt.Sprintf("%s - %s", it.SeriesName, rec.Name)
						} else {
							rec.Display = it.SeriesName
						}
					} else {
						if rec.Name != "" {
							rec.Display = rec.Name
						} else {
							rec.Display = fmt.Sprintf("Unknown Item (%s)", it.ID)
						}
					}
					base[it.ID] = rec
					// Upsert minimal metadata
					_, _ = db.Exec(`
                        INSERT INTO library_item (id, server_id, name, media_type, updated_at)
                        VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
                        ON CONFLICT(id) DO UPDATE SET
                            name = CASE WHEN excluded.name <> '' THEN excluded.name ELSE name END,
                            media_type = CASE WHEN excluded.media_type <> '' THEN excluded.media_type ELSE media_type END,
                            updated_at = CURRENT_TIMESTAMP
                    `, it.ID, serverID, rec.Name, rec.Type)
				}
			}
		}

		// Build output in request order
		out := make([]ItemRow, 0, len(ids))
		for _, id := range ids {
			if r, ok := base[id]; ok {
				if r.Display == "" {
					if r.Name != "" {
						r.Display = r.Name
					} else {
						r.Display = fmt.Sprintf("Unknown Item (%s)", id)
					}
				}
				if r.Type == "" {
					r.Type = "Unknown"
				}
				out = append(out, r)
			} else {
				out = append(out, ItemRow{ID: id, Name: fmt.Sprintf("Missing Item (%s)", id), Type: "Unknown", Display: fmt.Sprintf("Missing Item (%s)", id)})
			}
		}
		return c.JSON(out)
	}
}

// zero-pad to 2 digits
func two(n int) string {
	if n < 0 {
		return "00"
	}
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	if n < 100 {
		d1 := n / 10
		d2 := n % 10
		return string(rune('0'+d1)) + string(rune('0'+d2))
	}
	// For numbers >= 100, just return as string (shouldn't happen for season/episode numbers)
	return string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
}
