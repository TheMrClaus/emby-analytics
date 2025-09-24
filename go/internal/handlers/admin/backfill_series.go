package admin

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"emby-analytics/internal/emby"
	"emby-analytics/internal/media"
	"github.com/gofiber/fiber/v3"
)

type missingEpisode struct {
	StoredID   string
	RemoteID   string
	ServerID   string
	ServerType media.ServerType
	Name       string
}

type episodeBundle struct {
	ServerID   string
	ServerType media.ServerType
	RemoteIDs  []string
	Items      map[string]*missingEpisode
}

// BackfillSeries populates series_id/series_name for episodes missing linkage
// GET: dry-run summary; POST: apply updates.
func BackfillSeries(db *sql.DB, em *emby.Client, mgr *media.MultiServerManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		if mgr == nil && em == nil {
			return c.Status(500).JSON(fiber.Map{"error": "No media clients configured"})
		}
		method := string(c.Request().Header.Method())
		apply := method == fiber.MethodPost

		configs := map[string]media.ServerConfig{}
		if mgr != nil {
			configs = mgr.GetServerConfigs()
		}

		defaultEmbyID := ""
		for id, cfg := range configs {
			if cfg.Type == media.ServerTypeEmby {
				defaultEmbyID = id
				break
			}
		}

		rows, err := db.Query(`
			SELECT id, COALESCE(server_id, ''), COALESCE(server_type, ''), COALESCE(item_id, ''), COALESCE(name, '')
			FROM library_item
			WHERE media_type = 'Episode'
			  AND (series_id IS NULL OR TRIM(series_id) = '')
			LIMIT 500
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		bundles := map[string]*episodeBundle{}
		totalPending := 0
		for rows.Next() {
			var storedID, serverID, serverTypeRaw, remoteID, name string
			if err := rows.Scan(&storedID, &serverID, &serverTypeRaw, &remoteID, &name); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			storedID = strings.TrimSpace(storedID)
			serverID = strings.TrimSpace(serverID)
			remoteID = normalizeRemoteID(serverID, storedID, remoteID)
			if serverID == "" {
				serverID = defaultEmbyID
			}
			if serverID == "" {
				serverID = "default-emby"
			}
			cfg, hasCfg := configs[serverID]
			serverType := media.ServerType(strings.ToLower(strings.TrimSpace(serverTypeRaw)))
			if hasCfg {
				serverType = cfg.Type
			}
			if remoteID == "" {
				continue
			}
			bundle, ok := bundles[serverID]
			if !ok {
				bundle = &episodeBundle{
					ServerID:   serverID,
					ServerType: serverType,
					Items:      make(map[string]*missingEpisode),
				}
				bundles[serverID] = bundle
			}
			if _, exists := bundle.Items[remoteID]; !exists {
				bundle.RemoteIDs = append(bundle.RemoteIDs, remoteID)
			}
			bundle.Items[remoteID] = &missingEpisode{
				StoredID:   storedID,
				RemoteID:   remoteID,
				ServerID:   serverID,
				ServerType: bundle.ServerType,
				Name:       name,
			}
			totalPending++
		}
		if totalPending == 0 {
			return c.JSON(fiber.Map{"updated": 0, "pending": 0})
		}

		updated := 0
		errors := []string{}
		for serverID, bundle := range bundles {
			clientItems := []media.MediaItem{}
			if mgr != nil {
				if client, ok := mgr.GetClient(serverID); ok && client != nil {
					items, err := client.ItemsByIDs(bundle.RemoteIDs)
					if err != nil {
						errors = append(errors, fmt.Sprintf("%s: %v", serverID, err))
						continue
					}
					clientItems = append(clientItems, items...)
				}
			}
			if len(clientItems) == 0 && em != nil && (bundle.ServerType == "" || bundle.ServerType == media.ServerTypeEmby || strings.HasPrefix(serverID, "default-")) {
				emItems, err := em.ItemsByIDs(bundle.RemoteIDs)
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", serverID, err))
					continue
				}
				for _, it := range emItems {
					clientItems = append(clientItems, media.MediaItem{
						ID:                normalizeRemoteID(serverID, "", it.Id),
						Name:              it.Name,
						Type:              it.Type,
						SeriesID:          it.SeriesId,
						SeriesName:        it.SeriesName,
						ParentIndexNumber: it.ParentIndexNumber,
						IndexNumber:       it.IndexNumber,
					})
				}
			}
			if len(clientItems) == 0 {
				errors = append(errors, fmt.Sprintf("%s: unable to resolve metadata for %d episode(s)", serverID, len(bundle.Items)))
				continue
			}

			processed := make(map[string]bool)
			for _, item := range clientItems {
				remoteID := normalizeRemoteID(bundle.ServerID, "", item.ID)
				m := bundle.Items[remoteID]
				if m == nil {
					// try case-insensitive match
					for key, candidate := range bundle.Items {
						if strings.EqualFold(key, remoteID) {
							m = candidate
							break
						}
					}
				}
				if m == nil {
					continue
				}
				if processed[m.StoredID] {
					continue
				}
				if item.Type != "" && !strings.EqualFold(item.Type, "Episode") {
					continue
				}
				sid := strings.TrimSpace(item.SeriesID)
				sname := strings.TrimSpace(item.SeriesName)
				if sid == "" && sname != "" && em != nil && (bundle.ServerType == media.ServerTypeEmby || strings.HasPrefix(serverID, "default-")) {
					if seriesID, _ := em.FindSeriesIDByName(sname); seriesID != "" {
						sid = seriesID
					}
				}
				if sid == "" && sname == "" {
					continue
				}
				if apply {
					res, err := db.Exec(`UPDATE library_item SET series_id = COALESCE(?, series_id), series_name = COALESCE(?, series_name) WHERE id = ?`, nullIfEmpty(sid), nullIfEmpty(sname), m.StoredID)
					if err != nil {
						errors = append(errors, fmt.Sprintf("%s/%s: %v", serverID, m.StoredID, err))
						continue
					}
					if rows, rerr := res.RowsAffected(); rerr == nil && rows > 0 {
						updated++
					} else if rerr != nil {
						errors = append(errors, fmt.Sprintf("%s/%s: %v", serverID, m.StoredID, rerr))
					}
					if sid != "" {
						_, _ = db.Exec(`
	                        INSERT INTO series (id, name, year, created_at, updated_at)
	                        VALUES (?, ?, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	                        ON CONFLICT(id) DO UPDATE SET
	                            name = COALESCE(excluded.name, series.name),
	                            updated_at = CURRENT_TIMESTAMP
	                    `, sid, nullIfEmpty(sname))
					}
				} else {
					updated++
				}
				processed[m.StoredID] = true
			}
		}

		resp := fiber.Map{
			"updated": updated,
			"pending": totalPending,
			"applied": apply,
		}
		if len(errors) > 0 {
			resp["errors"] = errors
			if apply {
				log.Printf("[admin/backfill-series] Completed with errors: %v", errors)
			}
		}
		return c.JSON(resp)
	}
}

func normalizeRemoteID(serverID, storedID, candidate string) string {
	val := strings.TrimSpace(candidate)
	if val == "" {
		val = strings.TrimSpace(storedID)
	}
	if val == "" {
		return ""
	}
	if idx := strings.Index(val, "::"); idx >= 0 {
		val = val[idx+2:]
	}
	return strings.TrimSpace(val)
}
