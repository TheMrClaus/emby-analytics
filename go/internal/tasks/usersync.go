package tasks

import (
	"database/sql"
	"strings"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/handlers/settings"
	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
)

// StartUserSyncLoop handles the periodic background user sync across servers.
func StartUserSyncLoop(db *sql.DB, mgr *media.MultiServerManager, cfg config.Config) {
	if cfg.UserSyncIntervalSec <= 0 {
		logging.Debug("user sync loop disabled (interval <= 0)")
		return
	}
	interval := time.Duration(cfg.UserSyncIntervalSec) * time.Second
	logging.Debug("Starting user sync loop", "interval", interval)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			<-ticker.C
			runUserSync(db, mgr)
		}
	}()
}

// RunUserSyncOnce executes a single user sync cycle immediately.
func RunUserSyncOnce(db *sql.DB, mgr *media.MultiServerManager) {
	runUserSync(db, mgr)
}

func runUserSync(db *sql.DB, mgr *media.MultiServerManager) {
	configs := mgr.GetServerConfigs()
	clients := mgr.GetAllClients()
	if len(clients) == 0 {
		logging.Debug("user sync skipped: no media servers registered")
		return
	}

	start := time.Now()
	totalUsers := 0

	for serverID, client := range clients {
		if client == nil {
			continue
		}
		sc, ok := configs[serverID]
		if !ok {
			continue
		}
		if !shouldSyncServer(db, sc) {
			logging.Debug("user sync disabled for server", "server", sc.Name, "server_id", sc.ID)
			continue
		}

		processed := syncServerUsers(db, client, sc)
		totalUsers += processed
	}

	logging.Debug("user sync completed", "duration", time.Since(start).Round(time.Millisecond), "servers", len(clients), "users_processed", totalUsers)
}

func syncServerUsers(db *sql.DB, client media.MediaServerClient, sc media.ServerConfig) int {
	users, err := client.GetUsers()
	if err != nil {
		logging.Debug("user sync: failed to fetch users", "server", sc.Name, "error", err)
		return 0
	}

	processed := 0
	for _, u := range users {
		remoteID := strings.TrimSpace(u.ID)
		if remoteID == "" {
			continue
		}
		storedID := storageUserID(sc.ID, remoteID)
		_, err := db.Exec(`
			INSERT INTO emby_user (id, server_id, server_type, name)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name,
				server_id = excluded.server_id,
				server_type = excluded.server_type
		`, storedID, sc.ID, string(sc.Type), u.Name)
		if err != nil {
			logging.Debug("user sync: failed to upsert user", "server", sc.Name, "user", u.Name, "error", err)
			continue
		}
		processed++
		syncUserWatchData(db, client, sc, remoteID, storedID, u.Name)
	}

	// Mark users that no longer exist on server as deleted
	markDeletedUsers(db, sc.ID, users)

	return processed
}

// markDeletedUsers sets deleted_at for users in DB that are not in the server's user list
func markDeletedUsers(db *sql.DB, serverID string, liveUsers []media.User) {
	// Build set of live user IDs
	liveSet := make(map[string]bool)
	for _, u := range liveUsers {
		storedID := storageUserID(serverID, u.ID)
		liveSet[storedID] = true
	}

	// Get all users for this server from DB
	rows, err := db.Query(`SELECT id FROM emby_user WHERE server_id = ? AND deleted_at IS NULL`, serverID)
	if err != nil {
		logging.Debug("markDeletedUsers: failed to query users", "server_id", serverID, "error", err)
		return
	}
	defer rows.Close()

	var toDelete []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if !liveSet[id] {
			toDelete = append(toDelete, id)
		}
	}

	// Mark as deleted
	for _, id := range toDelete {
		_, err := db.Exec(`UPDATE emby_user SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
		if err != nil {
			logging.Debug("markDeletedUsers: failed to mark user as deleted", "user_id", id, "error", err)
		} else {
			logging.Debug("markDeletedUsers: marked user as deleted", "user_id", id)
		}
	}

	// Clear deleted_at for users that exist again (re-added)
	for id := range liveSet {
		_, _ = db.Exec(`UPDATE emby_user SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, id)
	}
}

func syncUserWatchData(db *sql.DB, client media.MediaServerClient, sc media.ServerConfig, remoteUserID, storedUserID, userName string) {
	items, err := client.GetUserData(remoteUserID)
	if err != nil {
		logging.Debug("user sync: failed to get watch data", "server", sc.Name, "user", userName, "error", err)
		return
	}

	includeTrakt := settings.GetSettingBool(db, "include_trakt_items", false)

	var embyWatchMs, traktWatchMs, totalWatchMs int64
	var traktItems, embyItems int

	for _, item := range items {
		if !item.Played || item.RuntimeMs <= 0 {
			continue
		}
		// Detect Trakt-synced entries (no playback evidence)
		hasLastPlayed := strings.TrimSpace(item.LastPlayed) != ""
		hasPlaybackPosition := item.PlaybackPositionMs > 0
		hasPlayCount := item.PlayCount > 0
		isTrakt := !hasLastPlayed && !hasPlaybackPosition && !hasPlayCount

		watchTimeMs := item.RuntimeMs

		if isTrakt {
			traktItems++
			traktWatchMs += watchTimeMs
			if includeTrakt {
				totalWatchMs += watchTimeMs
			}
		} else {
			embyItems++
			embyWatchMs += watchTimeMs
			totalWatchMs += watchTimeMs
		}

		// Ensure library item metadata is present for aggregated stats
		upsertUserAndItem(db, sc.ID, sc.Type, remoteUserID, userName, item.ID, item.Name, item.Type)
	}

	if traktItems > 0 || embyItems > 0 {
		logging.Debug("[usersync] processed user items",
			"user", userName,
			"server", sc.Name,
			"emby_items", embyItems,
			"trakt_items", traktItems,
			"include_trakt", includeTrakt)
	}

	_, err = db.Exec(`
		INSERT INTO lifetime_watch (user_id, total_ms, emby_ms, trakt_ms)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			total_ms = excluded.total_ms,
			emby_ms = excluded.emby_ms,
			trakt_ms = excluded.trakt_ms
	`, storedUserID, totalWatchMs, embyWatchMs, traktWatchMs)
	if err != nil {
		logging.Debug("user sync: failed to update lifetime watch", "server", sc.Name, "user", userName, "error", err)
	}
}
