package tasks

import (
	"database/sql"
	"log"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
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
	users, err := em.GetUsers()
	if err != nil {
		log.Printf("[usersync] ERROR fetching users: %v", err)
		return
	}

	upserted := 0
	for _, user := range users {
		res, err := db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                   ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			user.Id, user.Name)
		if err != nil {
			log.Printf("[usersync] ERROR upserting user %s: %v", user.Name, err)
			continue
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			upserted++
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
	var 