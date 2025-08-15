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
		_, err := db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                   ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			user.Id, user.Name)
		if err != nil {
			log.Printf("[usersync] error upserting user %s: %v", user.Id, err)
			continue
		}
		upserted++
	}

	log.Printf("[usersync] upserted %d users", upserted)
}
