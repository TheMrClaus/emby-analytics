package tasks

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/handlers/settings"
)

// StartUserSyncLoop now ONLY handles the periodic background syncs.
func StartUserSyncLoop(db *sql.DB, em *emby.Client, cfg config.Config) {
	if cfg.UserSyncIntervalSec <= 0 {
		logging.Debug("Periodic sync disabled (interval <= 0).")
		return
	}

	interval := time.Duration(cfg.UserSyncIntervalSec) * time.Second
	logging.Debug("Starting periodic loop with interval %v.", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C // This will wait for the first interval before running.
		runUserSync(db, em)
	}
}

// runUserSync is a private helper to perform the sync logic.
func runUserSync(db *sql.DB, em *emby.Client) {
	logging.Debug("starting periodic user sync...")
	startTime := time.Now()

	// DEBUG: Test the API connection first
	logging.Debug("Attempting to fetch users from Emby API...")

	users, err := em.GetUsers()
	if err != nil {
		logging.Debug("ERROR fetching users from Emby API: %v", err)
		return
	}

	logging.Debug("Successfully fetched %d users from Emby API", len(users))

	// DEBUG: Print details of each user
	for i, user := range users {
		logging.Debug("User %d: ID=%s, Name=%s", i+1, user.Id, user.Name)
	}

	upserted := 0
	for _, user := range users {
		logging.Debug("Processing user: %s (ID: %s)", user.Name, user.Id)

		res, err := db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                   ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			user.Id, user.Name)
		if err != nil {
			logging.Debug("ERROR upserting user %s: %v", user.Name, err)
			continue
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			upserted++
			logging.Debug("Successfully upserted user: %s", user.Name)
		}
		syncUserWatchData(db, em, user.Id, user.Name)
	}

	var totalInDB int
	_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&totalInDB)
	logging.Debug("periodic sync completed in %v: upserted %d users, total in DB: %d", time.Since(startTime), upserted, totalInDB)
}

func syncUserWatchData(db *sql.DB, em *emby.Client, userID, userName string) {
	userDataItems, err := em.GetUserData(userID)
	if err != nil {
		logging.Debug("failed to get watch data for %s: %v", userName, err)
		return
	}

	// Check if Trakt-synced items should be included for backward compatibility
	includeTrakt := settings.GetSettingBool(db, "include_trakt_items", false)

	var embyWatchMs, traktWatchMs, totalWatchMs int64
	var traktItems, embyItems int

	for _, item := range userDataItems {
		if item.UserData.Played && item.RunTimeTicks > 0 {
			// Better Trakt detection: Trakt-synced items typically have:
			// - Played=true (marked as watched)
			// - PlayCount=0 (never actually streamed through Emby)
			// - LastPlayedDate="" (no actual playback timestamp)
			// - PlaybackPositionTicks=0 (no resume position)

			hasLastPlayedDate := item.UserData.LastPlayedDate != ""
			hasPlaybackPosition := item.UserData.PlaybackPos > 0
			hasPlayCount := item.UserData.PlayCount > 0

			// Consider it Trakt-synced if it's marked played but has no Emby streaming evidence
			isTraktSynced := !hasLastPlayedDate && !hasPlaybackPosition && !hasPlayCount

			watchTimeMs := item.RunTimeTicks / 10000

			if isTraktSynced {
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
		}
	}

	// Log the breakdown for debugging
	if traktItems > 0 || embyItems > 0 {
		logging.Debug("[usersync] %s: %d Emby items (%dms), %d Trakt items (%dms), includeTrakt=%v",
			userName, embyItems, embyWatchMs, traktItems, traktWatchMs, includeTrakt)
	}

	// Store all three values: separate breakdown plus total for backward compatibility
	_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms, emby_ms, trakt_ms)
	                  VALUES (?, ?, ?, ?)
	                  ON CONFLICT(user_id) DO UPDATE SET 
	                      total_ms = excluded.total_ms,
	                      emby_ms = excluded.emby_ms,
	                      trakt_ms = excluded.trakt_ms`,
		userID, totalWatchMs, embyWatchMs, traktWatchMs)
	if err != nil {
		logging.Debug("failed to update lifetime watch for %s: %v", userName, err)
	}
}

// RunUserSyncOnce is the exported function for synchronous, on-demand syncs.
func RunUserSyncOnce(db *sql.DB, em *emby.Client) {
	runUserSync(db, em)
}
