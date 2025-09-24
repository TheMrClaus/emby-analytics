package tasks

import (
	"database/sql"
	"strings"
	"time"

	"emby-analytics/internal/jellyfin"
	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
	"emby-analytics/internal/plex"
)

const librarySyncSettingPrefix = "library_sync_at_"

// IngestLibraries pulls library metadata from external servers so stats endpoints can operate on up-to-date data.
func IngestLibraries(db *sql.DB, mgr *media.MultiServerManager, include map[string]bool) {
	if mgr == nil {
		return
	}
	configs := mgr.GetServerConfigs()
	clients := mgr.GetAllClients()
	for serverID, client := range clients {
		if client == nil {
			continue
		}
		if include != nil && !include[serverID] {
			continue
		}
		sc, ok := configs[serverID]
		if !ok {
			continue
		}
		if !shouldRunLibraryIngest(db, serverID, 6*time.Hour) {
			continue
		}
		var err error
		switch sc.Type {
		case media.ServerTypeJellyfin:
			if jf, ok := client.(*jellyfin.Client); ok {
				err = ingestJellyfinLibrary(db, sc, jf)
			}
		case media.ServerTypePlex:
			if px, ok := client.(*plex.Client); ok {
				err = ingestPlexLibrary(db, sc, px)
			}
		default:
			continue
		}
		if err != nil {
			logging.Debug("library ingest failed", "server", sc.Name, "server_id", sc.ID, "error", err)
			continue
		}
		_ = setSettingValue(db, librarySyncSettingPrefix+serverID, time.Now().UTC().Format(time.RFC3339))
	}
}

func shouldRunLibraryIngest(db *sql.DB, serverID string, interval time.Duration) bool {
	if interval <= 0 {
		return true
	}
	value, err := getSettingValue(db, librarySyncSettingPrefix+serverID)
	if err != nil || strings.TrimSpace(value) == "" {
		return true
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return time.Since(ts) >= interval
	}
	return true
}

func ingestJellyfinLibrary(db *sql.DB, sc media.ServerConfig, client *jellyfin.Client) error {
	items, err := client.FetchLibraryItems([]string{"Movie", "Episode"})
	if err != nil {
		return err
	}
	return upsertMediaItems(db, sc, items)
}

func ingestPlexLibrary(db *sql.DB, sc media.ServerConfig, client *plex.Client) error {
	items, err := client.FetchLibraryMovies()
	if err != nil {
		return err
	}
	return upsertMediaItems(db, sc, items)
}

func upsertMediaItems(db *sql.DB, sc media.ServerConfig, items []media.MediaItem) error {
	seriesUpserts := make(map[string]string)
	for _, item := range items {
		storedID := storageItemID(sc.ID, item.ID)
		if strings.TrimSpace(storedID) == "" {
			continue
		}
		var height interface{}
		if item.Height != nil {
			height = item.Height
		}
		var width interface{}
		if item.Width != nil {
			width = item.Width
		} else if item.Height != nil {
			calculated := int(float64(*item.Height) * 16.0 / 9.0)
			width = &calculated
		}
		var runtimeTicks interface{}
		if item.RuntimeMs != nil {
			ticks := *item.RuntimeMs * 10000
			runtimeTicks = &ticks
		}
		var genres interface{}
		if len(item.Genres) > 0 {
			joined := strings.Join(item.Genres, ", ")
			genres = joined
		}
		if sid := strings.TrimSpace(item.SeriesID); sid != "" {
			trimmedName := strings.TrimSpace(item.SeriesName)
			if existing, ok := seriesUpserts[sid]; !ok || (strings.TrimSpace(existing) == "" && trimmedName != "") {
				seriesUpserts[sid] = trimmedName
			}
		}

		_, err := db.Exec(`
			INSERT INTO library_item (id, server_id, server_type, item_id, name, media_type, height, width, run_time_ticks, container, video_codec, file_size_bytes, bitrate_bps, genres, series_id, series_name, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				server_id = COALESCE(excluded.server_id, library_item.server_id),
				server_type = COALESCE(excluded.server_type, library_item.server_type),
				item_id = COALESCE(excluded.item_id, library_item.item_id),
				name = COALESCE(excluded.name, library_item.name),
				media_type = COALESCE(excluded.media_type, library_item.media_type),
				height = COALESCE(excluded.height, library_item.height),
				width = COALESCE(excluded.width, library_item.width),
				run_time_ticks = COALESCE(excluded.run_time_ticks, library_item.run_time_ticks),
				container = COALESCE(excluded.container, library_item.container),
				video_codec = COALESCE(excluded.video_codec, library_item.video_codec),
				file_size_bytes = COALESCE(excluded.file_size_bytes, library_item.file_size_bytes),
				bitrate_bps = COALESCE(excluded.bitrate_bps, library_item.bitrate_bps),
				genres = COALESCE(NULLIF(excluded.genres, ''), library_item.genres),
				series_id = COALESCE(NULLIF(excluded.series_id, ''), library_item.series_id),
				series_name = COALESCE(NULLIF(excluded.series_name, ''), library_item.series_name),
				updated_at = CURRENT_TIMESTAMP
		`, storedID, sc.ID, string(sc.Type), item.ID, item.Name, item.Type, height, width, runtimeTicks, item.Container, item.Codec, item.FileSizeBytes, item.BitrateBps, genres, blankToNil(item.SeriesID), blankToNil(item.SeriesName))
		if err != nil {
			return err
		}
	}

	for sid, sname := range seriesUpserts {
		_, err := db.Exec(`
			INSERT INTO series (id, name, year, created_at, updated_at)
			VALUES (?, ?, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				name = COALESCE(NULLIF(excluded.name, ''), series.name),
				updated_at = CURRENT_TIMESTAMP
		`, sid, blankToNil(sname))
		if err != nil {
			return err
		}
	}
	return nil
}

func blankToNil(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func getSettingValue(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}
