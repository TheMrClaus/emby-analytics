package tasks

import (
	"database/sql"
	"log"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/handlers/settings"
)

// StartUserSyncLoop now ONLY handles the periodic background syncs.
func StartUserSyncLoop(db *sql.DB, em *emby.Client, cfg config.Config) {
	if cfg.UserSyncIntervalSec <= 0 {
		log.Println("[usersync] Periodic sync disabled (interval <= 0).")
		return
	}

	interval := time.Duration(cfg.UserSyncIntervalSec) * time.Second
	log.Printf("[usersync] Starting periodic loop with interval %v.", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C // This will wait for the first interval before running.
		runUserSync(db, em)
	}
}

// runUserSync is a private helper to perform the sync logic.
func runUserSync(db *sql.DB, em *emby.Client) {
	log.Println("[usersync] starting periodic user sync...")
	startTime := time.Now()

	// DEBUG: Test the API connection first
	log.Printf("[usersync] Attempting to fetch users from Emby API...")

	users, err := em.GetUsers()
	if err != nil {
		log.Printf("[usersync] ERROR fetching users from Emby API: %v", err)
		return
	}

	log.Printf("[usersync] Successfully fetched %d users from Emby API", len(users))

	// DEBUG: Print details of each user
	for i, user := range users {
		log.Printf("[usersync] User %d: ID=%s, Name=%s", i+1, user.Id, user.Name)
	}

	upserted := 0
	for _, user := range users {
		log.Printf("[usersync] Processing user: %s (ID: %s)", user.Name, user.Id)

		res, err := db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                   ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			user.Id, user.Name)
		if err != nil {
			log.Printf("[usersync] ERROR upserting user %s: %v", user.Name, err)
			continue
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			upserted++
			log.Printf("[usersync] Successfully upserted user: %s", user.Name)
		}
		syncUserWatchData(db, em, user.Id, user.Name)
	}

	var totalInDB int
	_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&totalInDB)
	log.Printf("[usersync] periodic sync completed in %v: upserted %d users, total in DB: %d", time.Since(startTime), upserted, totalInDB)
}

func syncUserWatchData(db *sql.DB, em *emby.Client, userID, userName string) {
	userDataItems, err := em.GetUserData(userID)
	if err != nil {
		log.Printf("[usersync] failed to get watch data for %s: %v", userName, err)
		return
	}

	// Check if Trakt-synced items should be included
	includeTrakt := settings.GetSettingBool(db, "include_trakt_items", false)
	
	var totalWatchMs int64
	var traktItems, embyItems int
	
	for _, item := range userDataItems {
		if item.UserData.Played && item.RunTimeTicks > 0 {
			// Detect Trakt-synced items: Played=true but PlayCount=0
			isTraktSynced := item.UserData.PlayCount == 0
			
			if isTraktSynced {
				traktItems++
				if !includeTrakt {
					continue // Skip Trakt-synced items if setting is disabled
				}
			} else {
				embyItems++
			}
			
			totalWatchMs += item.RunTimeTicks / 10000
		}
	}
	
	// Log the breakdown for debugging
	if traktItems > 0 || embyItems > 0 {
		log.Printf("[usersync] %s: %d Emby items, %d Trakt items, includeTrakt=%v", userName, embyItems, traktItems, includeTrakt)
	}
	
	_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms)
	                  VALUES (?, ?)
	                  ON CONFLICT(user_id) DO UPDATE SET total_ms = excluded.total_ms`,
		userID, totalWatchMs)
	if err != nil {
		log.Printf("[usersync] failed to update lifetime watch for %s: %v", userName, err)
	}
}

// RunUserSyncOnce is the exported function for synchronous, on-demand syncs.
func RunUserSyncOnce(db *sql.DB, em *emby.Client) {
	runUserSync(db, em)
}
