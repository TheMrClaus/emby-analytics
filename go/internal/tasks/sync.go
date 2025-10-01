package tasks

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/handlers/settings"
	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
)

// StartSyncLoop launches a background ticker that periodically synchronizes
// play history from all enabled media servers registered with the manager.
func StartSyncLoop(db *sql.DB, mgr *media.MultiServerManager, cfg config.Config) {
	interval := time.Duration(cfg.SyncIntervalSec) * time.Second
	if interval <= 0 {
		logging.Debug("play history sync loop disabled (interval <= 0)")
		return
	}
	logging.Debug("starting play sync loop with interval %v", interval)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			runSyncFiltered(db, mgr, cfg, nil, nil)
			<-ticker.C
		}
	}()
}

// RunOnce triggers a single synchronization cycle immediately.
func RunOnce(db *sql.DB, mgr *media.MultiServerManager, cfg config.Config) {
	IngestLibraries(db, mgr, nil, nil)
	runSyncFiltered(db, mgr, cfg, nil, nil)
}

// RunServerOnce synchronizes a specific server immediately.
// It will force the sync even if the server is disabled in settings; callers should guard as needed.
func RunServerOnce(db *sql.DB, mgr *media.MultiServerManager, cfg config.Config, serverID string) error {
	if strings.TrimSpace(serverID) == "" {
		return fmt.Errorf("server id is required")
	}
	configs := mgr.GetServerConfigs()
	if _, ok := configs[serverID]; !ok {
		return fmt.Errorf("server %s not found", serverID)
	}
	filter := map[string]bool{serverID: true}
	force := map[string]bool{serverID: true}
	IngestLibraries(db, mgr, filter, force)
	if isSyncDisabled(db, serverID, configs[serverID].Enabled) {
		CancelServerSyncProgress(serverID, "Sync cancelled by user")
		return nil
	}
	SetServerSyncStage(serverID, "Collecting playback history...")
	runSyncFiltered(db, mgr, cfg, filter, force)
	return nil
}

func runSyncFiltered(db *sql.DB, mgr *media.MultiServerManager, cfg config.Config, include map[string]bool, force map[string]bool) {
	configs := mgr.GetServerConfigs()
	clients := mgr.GetAllClients()
	if len(clients) == 0 {
		logging.Debug("play sync skipped: no media servers registered")
		return
	}

	totalInserted := 0
	totalAPICalls := 0
	start := time.Now()

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
		if force == nil || !force[serverID] {
			if !shouldSyncServer(db, sc) {
				logging.Debug("play sync disabled for server", "server", sc.Name, "server_id", sc.ID)
				continue
			}
		}
		if isSyncDisabled(db, serverID, sc.Enabled) {
			CancelServerSyncProgress(serverID, "Sync cancelled by user")
			continue
		}

		logging.Debug("play sync started", "server", sc.Name, "server_id", sc.ID)
		SetServerSyncStage(serverID, "Collecting playback history...")

		inserted, apiCalls, err := syncServer(db, client, sc, cfg)
		totalInserted += inserted
		totalAPICalls += apiCalls
		switch {
		case err == nil:
			CompleteServerSyncProgress(serverID)
		case errors.Is(err, ErrSyncCancelled):
			// Already marked as cancelled where detected.
		default:
			logging.Debug("play sync failed", "server", sc.Name, "server_id", sc.ID, "error", err)
			FailServerSyncProgress(serverID, err)
		}
	}

	dur := time.Since(start)
	if totalInserted > 0 || totalAPICalls > 0 {
		logging.Debug("play sync completed", "duration", dur.Round(time.Millisecond), "api_calls", totalAPICalls, "events", totalInserted)
	}
}

func shouldSyncServer(db *sql.DB, sc media.ServerConfig) bool {
	return settings.GetSyncEnabled(db, sc.ID, sc.Enabled)
}

func syncServer(db *sql.DB, client media.MediaServerClient, sc media.ServerConfig, cfg config.Config) (int, int, error) {
	serverID := client.GetServerID()
	serverType := client.GetServerType()
	serverName := client.GetServerName()

	insertedEvents := 0
	apiCalls := 0

	checkCancelled := func() bool {
		return isSyncDisabled(db, serverID, sc.Enabled)
	}

	// Determine if this is the first sync for the server
	isInitialized := settings.GetSettingBool(db, syncInitializedKey(serverID), false)
	historyDays := cfg.HistoryDays
	if !isInitialized {
		historyDays = 0 // fetch full history on first sync
		logging.Debug("First sync detected for server", "server", serverName, "server_id", serverID)
	}
	if checkCancelled() {
		CancelServerSyncProgress(serverID, "Sync cancelled by user")
		return insertedEvents, apiCalls, ErrSyncCancelled
	}

	// Step 1: Active sessions snapshot
	sessions, err := client.GetActiveSessions()
	apiCalls++
	if err != nil {
		logging.Debug("play sync: failed to fetch sessions", "server", serverName, "error", err)
	} else {
		for idx, s := range sessions {
			if idx%cancelCheckInterval == 0 && checkCancelled() {
				CancelServerSyncProgress(serverID, "Sync cancelled by user")
				return insertedEvents, apiCalls, ErrSyncCancelled
			}
			upsertUserAndItem(db, serverID, serverType, s.UserID, s.UserName, s.ItemID, s.ItemName, s.ItemType)

			posMs := clampPositionMs(s.PositionMs, s.DurationMs)
			storedUserID := storageUserID(serverID, s.UserID)
			storedItemID := storageItemID(serverID, s.ItemID)
			if insertPlayEvent(db, storedUserID, storedItemID, posMs) {
				insertedEvents++
			}
		}
	}

	// Step 2: user history backfill
	users, err := client.GetUsers()
	apiCalls++
	if err != nil {
		logging.Debug("play sync: failed to fetch users", "server", serverName, "error", err)
		return insertedEvents, apiCalls, err
	}

	for idx, user := range users {
		if idx%cancelCheckInterval == 0 && checkCancelled() {
			CancelServerSyncProgress(serverID, "Sync cancelled by user")
			return insertedEvents, apiCalls, ErrSyncCancelled
		}
		remoteUserID := user.ID
		if strings.TrimSpace(remoteUserID) == "" {
			continue
		}

		storedUserID := storageUserID(serverID, remoteUserID)
		upsertUserAndItem(db, serverID, serverType, remoteUserID, user.Name, "", "", "")

		history, err := client.GetUserPlayHistory(remoteUserID, historyDays)
		apiCalls++
		if err != nil {
			logging.Debug("play sync: history fetch failed", "server", serverName, "user", user.Name, "error", err)
			continue
		}

		for hIdx, h := range history {
			if hIdx%cancelCheckInterval == 0 && checkCancelled() {
				CancelServerSyncProgress(serverID, "Sync cancelled by user")
				return insertedEvents, apiCalls, ErrSyncCancelled
			}
			if strings.TrimSpace(h.ID) == "" {
				continue
			}
			upsertUserAndItem(db, serverID, serverType, remoteUserID, user.Name, h.ID, h.Name, h.Type)

			storedItemID := storageItemID(serverID, h.ID)
			eventTime := parseEventTime(h.DatePlayed)
			posMs := h.PlaybackPos
			if posMs < 0 {
				posMs = 0
			}
			if insertPlayEventWithTimestamp(db, storedUserID, storedItemID, posMs, eventTime) {
				insertedEvents++
			}
		}
	}

	if !isInitialized {
		if err := setSettingValue(db, syncInitializedKey(serverID), "true"); err != nil {
			logging.Debug("play sync: failed to mark server initialized", "server", serverName, "error", err)
		}
	}

	return insertedEvents, apiCalls, nil
}

func clampPositionMs(posMs, durationMs int64) int64 {
	if posMs < 0 {
		return 0
	}
	if durationMs > 0 && posMs > durationMs {
		return durationMs
	}
	return posMs
}

func parseEventTime(value string) int64 {
	if strings.TrimSpace(value) == "" {
		return time.Now().UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UnixMilli()
	}
	if t, err := time.Parse("2006-01-02T15:04:05", value); err == nil {
		return t.UnixMilli()
	}
	return time.Now().UnixMilli()
}

func insertPlayEvent(db *sql.DB, userID, itemID string, posMs int64) bool {
	ts := time.Now().UnixMilli()
	res, err := db.Exec(`INSERT INTO play_event (ts, user_id, item_id, pos_ms) VALUES (?, ?, ?, ?)`, ts, userID, itemID, posMs)
	if err != nil {
		return false
	}
	rows, _ := res.RowsAffected()
	return rows > 0
}

func insertPlayEventWithTimestamp(db *sql.DB, userID, itemID string, posMs int64, timestamp int64) bool {
	res, err := db.Exec(`INSERT OR IGNORE INTO play_event (ts, user_id, item_id, pos_ms) VALUES (?, ?, ?, ?)`, timestamp, userID, itemID, posMs)
	if err != nil {
		return false
	}
	rows, _ := res.RowsAffected()
	return rows > 0
}

func upsertUserAndItem(db *sql.DB, serverID string, serverType media.ServerType, userID, userName, itemID, itemName, itemType string) {
	storedUserID := storageUserID(serverID, userID)
	if strings.TrimSpace(storedUserID) != "" {
		_, _ = db.Exec(`
			INSERT INTO emby_user (id, server_id, server_type, name)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name,
				server_id = excluded.server_id,
				server_type = excluded.server_type
		`, storedUserID, serverID, string(serverType), userName)
	}

	storedItemID := storageItemID(serverID, itemID)
	if strings.TrimSpace(storedItemID) != "" {
		_, _ = db.Exec(`
			INSERT INTO library_item (id, server_id, server_type, item_id, name, media_type, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				server_id = excluded.server_id,
				server_type = excluded.server_type,
				item_id = excluded.item_id,
				name = COALESCE(excluded.name, library_item.name),
				media_type = COALESCE(excluded.media_type, library_item.media_type),
				updated_at = CURRENT_TIMESTAMP
		`, storedItemID, serverID, string(serverType), itemID, itemName, itemType)
	}
}

func setSettingValue(db *sql.DB, key, value string) error {
	_, err := db.Exec(`
		INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().UTC())
	return err
}
