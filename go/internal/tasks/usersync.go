package tasks

import (
	"database/sql"
	"log"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
)

// StartUserSyncLoop runs the user sync task periodically
func StartUserSyncLoop(db *sql.DB, em *emby.Client, cfg config.Config) {
	if cfg.UserSyncIntervalSec <= 0 {
		log.Println("[usersync] disabled (interval <= 0)")
		return
	}

	ticker := time.NewTicker(time.Duration(cfg.UserSyncIntervalSec) * time.Second)
	defer ticker.Stop()

	log.Printf("[usersync] starting loop with interval %d seconds", cfg.UserSyncIntervalSec)

	// Run once immediately
	runUserSync(db, em)

	for {
		<-ticker.C
		runUserSync(db, em)
	}
}

func runUserSync(db *sql.DB, em *emby.Client) {
	users, err := em.GetUsers()
	if err != nil {
		log.Printf("[usersync] error fetching users: %v", err)
		return
	}

	upserted := 0
	for _, user := range users {
		// Upsert user
		_, err := db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                   ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			user.Id, user.Name)
		if err != nil {
			log.Printf("[usersync] error upserting user %s: %v", user.Id, err)
			continue
		}
		upserted++

		// Sync user watch data
		syncUserWatchData(db, em, user.Id, user.Name)
	}

	log.Printf("[usersync] upserted %d users", upserted)
}

func syncUserWatchData(db *sql.DB, em *emby.Client, userID, userName string) {
	userDataItems, err := em.GetUserData(userID)
	if err != nil {
		log.Printf("[usersync] failed to get user data for %s: %v", userName, err)
		return
	}

	// Calculate total watch time from completed items
	var totalWatchMs int64
	for _, item := range userDataItems {
		if item.UserData.Played && item.RunTimeTicks > 0 {
			// Convert ticks to milliseconds (1 tick = 100 nanoseconds)
			itemRuntimeMs := item.RunTimeTicks / 10000
			totalWatchMs += itemRuntimeMs
		}
	}

	// Update lifetime watch with calculated total
	_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms)
	                  VALUES (?, ?)
	                  ON CONFLICT(user_id) DO UPDATE SET total_ms = excluded.total_ms`,
		userID, totalWatchMs)

	if err != nil {
		log.Printf("[usersync] failed to update lifetime watch for %s: %v", userName, err)
	} else {
		hours := totalWatchMs / 3600000
		log.Printf("[usersync] updated %s: %d hours total watch time", userName, hours)
	}
}

// RunUserSyncOnce triggers a single user sync cycle immediately
func RunUserSyncOnce(db *sql.DB, em *emby.Client) {
	runUserSync(db, em)
}
