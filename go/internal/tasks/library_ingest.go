package tasks

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"emby-analytics/internal/jellyfin"
	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
	"emby-analytics/internal/plex"
)

const librarySyncSettingPrefix = "library_sync_at_"

// IngestLibraries pulls library metadata from external servers so stats endpoints can operate on up-to-date data.
func IngestLibraries(db *sql.DB, mgr *media.MultiServerManager, include map[string]bool, force map[string]bool) {
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
		forced := force != nil && force[serverID]
		if !forced && !shouldRunLibraryIngest(db, serverID, sc.Enabled, 6*time.Hour) {
			continue
		}

		StartServerSyncProgress(serverID, sc.Name)
		SetServerSyncStage(serverID, "Fetching library metadata...")
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
		case media.ServerTypeEmby:
			if em, ok := client.(*media.EmbyAdapter); ok {
				err = ingestEmbyLibrary(db, sc, em)
			}
		default:
			continue
		}
		if err != nil {
			if errors.Is(err, ErrSyncCancelled) {
				logging.Debug("library ingest cancelled", "server", sc.Name, "server_id", sc.ID)
			} else {
				logging.Debug("library ingest failed", "server", sc.Name, "server_id", sc.ID, "error", err)
				SetServerSyncStage(serverID, fmt.Sprintf("Sync failed: %v", err))
				FailServerSyncProgress(serverID, err)
			}
			continue
		}
		_ = setSettingValue(db, librarySyncSettingPrefix+serverID, time.Now().UTC().Format(time.RFC3339))
	}
}

func ingestEmbyLibrary(db *sql.DB, sc media.ServerConfig, client *media.EmbyAdapter) error {
	items, err := client.FetchLibraryItems()
	if err != nil {
		return err
	}
	if isSyncDisabled(db, sc.ID, sc.Enabled) {
		CancelServerSyncProgress(sc.ID, "Sync cancelled by user")
		return ErrSyncCancelled
	}
	UpdateServerSyncTotals(sc.ID, len(items))
	SetServerSyncProcessed(sc.ID, 0)
	if len(items) == 0 {
		SetServerSyncStage(sc.ID, "No library items returned")
		return nil
	}
	SetServerSyncStage(sc.ID, fmt.Sprintf("Ingesting %d items...", len(items)))
	return upsertMediaItems(db, sc, items)
}

func shouldRunLibraryIngest(db *sql.DB, serverID string, defaultEnabled bool, interval time.Duration) bool {
	if isSyncDisabled(db, serverID, defaultEnabled) {
		return false
	}
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
	if isSyncDisabled(db, sc.ID, sc.Enabled) {
		CancelServerSyncProgress(sc.ID, "Sync cancelled by user")
		return ErrSyncCancelled
	}
	UpdateServerSyncTotals(sc.ID, len(items))
	SetServerSyncProcessed(sc.ID, 0)
	if len(items) == 0 {
		SetServerSyncStage(sc.ID, "No library items returned")
		return nil
	}
	SetServerSyncStage(sc.ID, fmt.Sprintf("Ingesting %d items...", len(items)))
	return upsertMediaItems(db, sc, items)
}

func ingestPlexLibrary(db *sql.DB, sc media.ServerConfig, client *plex.Client) error {
	items, err := client.FetchLibraryItems()
	if err != nil {
		return err
	}
	if isSyncDisabled(db, sc.ID, sc.Enabled) {
		CancelServerSyncProgress(sc.ID, "Sync cancelled by user")
		return ErrSyncCancelled
	}
	UpdateServerSyncTotals(sc.ID, len(items))
	SetServerSyncProcessed(sc.ID, 0)
	if len(items) == 0 {
		SetServerSyncStage(sc.ID, "No library items returned")
		return nil
	}
	SetServerSyncStage(sc.ID, fmt.Sprintf("Ingesting %d items...", len(items)))
	return upsertMediaItems(db, sc, items)
}

func upsertMediaItems(db *sql.DB, sc media.ServerConfig, items []media.MediaItem) error {
	logging.Info("IngestLibraries: processing items", "fetched_count", len(items), "server", sc.Name)

	// Step 1: Get all existing IDs for this server to track deletions
	existingIDs, err := getAllLibraryItemIDs(db, sc.ID)
	if err != nil {
		logging.Debug("failed to fetch existing library items for deletion tracking", "server", sc.Name, "error", err)
		// We proceed anyway, but deletion will be disabled for this run to be safe
		existingIDs = nil
	} else {
		logging.Info("IngestLibraries: tracking deletions", "existing_db_count", len(existingIDs))
	}

	// Start transaction for bulk operations
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statements for performance
	upsertStmt, err := tx.Prepare(`
		INSERT INTO library_item (id, server_id, server_type, item_id, name, media_type, height, width, run_time_ticks, container, video_codec, file_size_bytes, bitrate_bps, file_path, genres, series_id, series_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
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
			file_path = COALESCE(NULLIF(excluded.file_path, ''), library_item.file_path),
			genres = COALESCE(NULLIF(excluded.genres, ''), library_item.genres),
			series_id = COALESCE(NULLIF(excluded.series_id, ''), library_item.series_id),
			series_name = COALESCE(NULLIF(excluded.series_name, ''), library_item.series_name),
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert statement: %w", err)
	}
	defer upsertStmt.Close()

	seriesUpserts := make(map[string]string)
	for idx, item := range items {
		if idx%cancelCheckInterval == 0 && isSyncDisabled(db, sc.ID, sc.Enabled) {
			CancelServerSyncProgress(sc.ID, "Sync cancelled by user")
			return ErrSyncCancelled
		}
		storedID := storageItemID(sc.ID, item.ID)
		if strings.TrimSpace(storedID) == "" {
			continue
		}

		// Mark as seen by removing from the existing set
		if existingIDs != nil {
			delete(existingIDs, storedID)
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

		_, err := upsertStmt.Exec(storedID, sc.ID, string(sc.Type), item.ID, item.Name, item.Type, height, width, runtimeTicks, item.Container, item.Codec, item.FileSizeBytes, item.BitrateBps, blankToNil(item.FilePath), genres, blankToNil(item.SeriesID), blankToNil(item.SeriesName))
		if err != nil {
			logging.Debug("failed to upsert item", "item_id", item.ID, "error", err)
			continue // Don't fail entire batch for one bad item
		}
		IncrementServerSyncProcessed(sc.ID, 1)
	}

	// Commit the upserts before moving to deletions
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit upserts: %w", err)
	}

	// Step 2: Delete items that were not found in the current sync
	if existingIDs != nil && len(existingIDs) > 0 {
		deletedCount := 0
		// Delete in batches prevents huge queries if many items are deleted
		batchSize := 50
		toDelete := make([]string, 0, batchSize)

		for id := range existingIDs {
			toDelete = append(toDelete, id)
			if len(toDelete) >= batchSize {
				if err := deleteLibraryItems(db, toDelete); err != nil {
					logging.Debug("failed to delete batch of stale items", "error", err)
				} else {
					deletedCount += len(toDelete)
				}
				toDelete = toDelete[:0]
			}
		}
		if len(toDelete) > 0 {
			if err := deleteLibraryItems(db, toDelete); err != nil {
				logging.Debug("failed to delete final batch of stale items", "error", err)
			} else {
				deletedCount += len(toDelete)
			}
		}
		if deletedCount > 0 {
			logging.Debug("pruned stale library items", "server", sc.Name, "count", deletedCount)
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
	SetServerSyncStage(sc.ID, fmt.Sprintf("Library ingest complete (%d items)", len(items)))
	SetServerSyncProcessed(sc.ID, len(items))
	return nil
}

func getAllLibraryItemIDs(db *sql.DB, serverID string) (map[string]bool, error) {
	rows, err := db.Query("SELECT id FROM library_item WHERE server_id = ?", serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func deleteLibraryItems(db *sql.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf("DELETE FROM library_item WHERE id IN (%s)", placeholders)

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err := db.Exec(query, args...)
	return err
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
